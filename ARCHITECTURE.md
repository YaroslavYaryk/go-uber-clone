# Architecture & Developer Guide

A detailed reference for every layer of the ride-sharing platform: services, message flows, API contracts, data models, and local setup.

---

## Table of contents

1. [System overview](#1-system-overview)
2. [Services](#2-services)
   - [API Gateway](#api-gateway)
   - [Trip Service](#trip-service)
   - [Driver Service](#driver-service)
   - [Payment Service](#payment-service)
3. [Frontend (web)](#3-frontend-web)
4. [Messaging: RabbitMQ](#4-messaging-rabbitmq)
   - [Exchange and topology](#exchange-and-topology)
   - [Routing keys](#routing-keys)
   - [Queue bindings](#queue-bindings)
5. [End-to-end flows](#5-end-to-end-flows)
   - [Trip preview and fare selection](#trip-preview-and-fare-selection)
   - [Starting a trip (driver matching)](#starting-a-trip-driver-matching)
   - [Driver accepts / declines](#driver-accepts--declines)
   - [Payment flow](#payment-flow)
   - [Trip cancellation](#trip-cancellation)
6. [gRPC contracts](#6-grpc-contracts)
7. [HTTP API](#7-http-api)
8. [WebSocket protocol](#8-websocket-protocol)
9. [Data models (MongoDB)](#9-data-models-mongodb)
10. [Shared packages](#10-shared-packages)
11. [Infrastructure and deployment](#11-infrastructure-and-deployment)
    - [Local development with Tilt](#local-development-with-tilt)
    - [Kubernetes manifests](#kubernetes-manifests)
    - [Production (GKE)](#production-gke)
12. [Technology reference](#12-technology-reference)

---

## 1. System overview

```
Browser (Next.js)
      │  HTTP REST                WebSocket (/ws/riders, /ws/drivers)
      ▼
┌─────────────────┐
│   api-gateway   │──gRPC──► trip-service ──► MongoDB
│   :8081 HTTP    │──gRPC──► driver-service (in-memory)
│                 │
│  RabbitMQ pub   │◄──────── RabbitMQ consumers (trip, driver, payment events)
└────────┬────────┘
         │  AMQP (topic exchange "trip")
    ─────┴──────────────────────────────────────
         │                │                │
    trip-service    driver-service   payment-service
         │                                 │
       MongoDB                           Stripe
```

All four services connect to the same RabbitMQ **topic exchange** named `trip`. Events are published with dot-separated routing keys (`trip.event.created`, `payment.event.success`, etc.), and each service declares its own durable queues bound to the routing keys it cares about.

Real-time updates reach the browser through the API Gateway's WebSocket consumers: when a RabbitMQ message arrives the gateway looks up the target user ID in the `ConnectionManager` and pushes a JSON frame.

---

## 2. Services

### API Gateway

**Path:** `services/api-gateway/`  
**Port:** `8081` (HTTP + WebSocket)

The single entry point for all client traffic. It owns:

- **HTTP handlers** (`http.go`) — trip preview, trip start, trip cancel, Stripe webhook
- **WebSocket handlers** (`ws.go`) — one endpoint for riders (`/ws/riders`), one for drivers (`/ws/drivers`)
- **RabbitMQ consumers** — listens on several queues and fan-outs events to the correct WebSocket connection via `ConnectionManager`
- **gRPC clients** (`internal/grpc_clients/`) — thin wrappers around the trip-service and driver-service gRPC stubs; a new connection is created per request so a downstream restart never blocks the gateway

On driver WebSocket connect, the gateway immediately calls `driver-service.RegisterDriver` over gRPC to add the driver to the in-memory pool. On disconnect it calls `UnRegisterDriver`.

The gateway forwards driver responses (`driver.cmd.trip_accept`, `driver.cmd.trip_decline`) from the WebSocket into RabbitMQ so the trip-service can act on them.

**Key env vars:**

| Variable | Default |
|---|---|
| `HTTP_ADDR` | `:8081` |
| `RABBITMQ_URI` | `amqp://guest:guest@localhost:5672/` |
| `TRIP_SERVICE_ADDR` | `trip-service:9093` |
| `DRIVER_SERVICE_ADDR` | `driver-service:9092` |
| `STRIPE_WEBHOOK_KEY` | _(required)_ |

---

### Trip Service

**Path:** `services/trip-service/`  
**Ports:** `9093` (gRPC), `8083` (HTTP/service port in k8s)

Owns all trip lifecycle logic. Responsibilities:

- **PreviewTrip** (gRPC) — calls the public [OSRM API](http://router.project-osrm.org) to get a driving route, computes fares for each car package (SUV, Sedan, Van, Luxury), saves `RiderFare` documents to MongoDB, and returns route + fares to the gateway
- **CreateTrip** (gRPC) — validates the chosen fare belongs to the requesting user, creates a `Trip` document in MongoDB with status `pending`, publishes `trip.event.created` to RabbitMQ
- **Driver consumer** (`events/driver_consumer.go`) — listens for `driver.cmd.trip_accept` / `driver.cmd.trip_decline`; on accept updates the trip's status and driver fields in MongoDB and publishes `trip.event.driver_assigned` + `payment.cmd.create_session`
- **Payment consumer** (`events/payment_consumer.go`) — listens for `payment.event.success`; updates trip status to `paid` in MongoDB and publishes `payment.event.success` back so the rider's WebSocket gets a confirmation

**Key env vars:**

| Variable | Default |
|---|---|
| `RABBITMQ_URI` | `amqp://guest:guest@localhost:5672/` |
| `MONGODB_URI` | `mongodb://localhost:27017` |

**Fare pricing model:**

Base prices per package (in cents):

| Package | Base price |
|---|---|
| `suv` | 200 |
| `sedan` | 350 |
| `van` | 400 |
| `luxury` | 1 000 |

Final fare = base + (distance_km × price_per_km) + (duration_min × price_per_min)

---

### Driver Service

**Path:** `services/driver-service/`  
**Port:** `9092` (gRPC)

Maintains an **in-memory** registry of connected drivers (no persistent store). Responsibilities:

- **RegisterDriver / UnRegisterDriver** (gRPC) — called by the gateway on WebSocket connect/disconnect; generates a fake driver profile with a random geohash location
- **FindAvailableDrivers** — returns driver IDs that match a given `packageSlug` (car type)
- **Trip consumer** (`trip_consumer.go`) — listens on `find_available_drivers`; picks the first matching driver, records the `tripID → driverID` mapping in a `pendingTrips` map (thread-safe with `sync.RWMutex`), and publishes `driver.cmd.trip_request` to the matched driver's WebSocket
- **Cancellation consumer** (`trip_consumer.go`) — listens on `trip_cancelled_driver`; looks up the pending driver for the cancelled trip and publishes `driver.cmd.trip_cancelled` to their WebSocket

Both consumers run sequentially in a single goroutine to avoid concurrent use of the same AMQP channel (the `amqp091-go` channel is not thread-safe).

**Key env vars:**

| Variable | Default |
|---|---|
| `RABBITMQ_URI` | `amqp://guest:guest@localhost:5672/` |

---

### Payment Service

**Path:** `services/payment-service/`

Handles all Stripe interactions. Responsibilities:

- **Trip consumer** (`internal/events/trip_consumer.go`) — listens for `payment.cmd.create_session`; creates a Stripe Checkout Session with the trip amount, embeds `trip_id`, `user_id`, `driver_id` in Stripe metadata, and publishes `payment.event.session_created` with the session URL
- The rider's browser receives the session URL via WebSocket and redirects to Stripe
- After the user completes payment, Stripe calls `POST /webhook/stripe` on the gateway; the gateway validates the signature and publishes `payment.event.success`
- The payment service does **not** consume `payment.event.success` directly — the trip-service and gateway consumers handle that

**Key env vars:**

| Variable | Default |
|---|---|
| `RABBITMQ_URI` | `amqp://guest:guest@rabbitmq:5672/` |
| `STRIPE_SECRET_KEY` | _(required)_ |
| `STRIPE_SUCCESS_URL` | `http://localhost:3000?payment=success` |
| `STRIPE_CANCEL_URL` | `http://localhost:3000?payment=cancel` |
| `APP_URL` | `http://localhost:3000` |

---

## 3. Frontend (web)

**Path:** `web/`  
**Stack:** Next.js 15, React 19, TypeScript, Tailwind CSS, Leaflet, Stripe.js

Two main map views:

| Route | Component | Role |
|---|---|---|
| `/` | `RiderMap` | Rider picks pickup/destination, selects fare, tracks driver, pays |
| `/driver` | `DriverMap` | Driver sees trip requests, accepts/declines, sees active trip |

**State management** — no global state library; each view has a corresponding hook:

- `useRiderStreamConnection` — manages the rider's WebSocket, handles all incoming events, owns `tripStatus` / `assignedDriver` state
- `useDriverStreamConnection` — manages the driver's WebSocket, owns `requestedTrip` state

**Payment redirect recovery** — when Stripe redirects back with `?payment=success` the React state is gone. The app persists `trip`, `destination`, and `assignedDriver` to `localStorage` before the redirect and restores them on mount if the URL contains `?payment=success`.

**Leaflet** is loaded with `next/dynamic` and `ssr: false` because it relies on browser APIs not available in Node SSR.

**Default map center:** Lviv, Ukraine (`49.8397, 24.0297`)

---

## 4. Messaging: RabbitMQ

### Exchange and topology

A single **topic exchange** named `trip` carries all messages. Every service that publishes or consumes calls `NewRabbitMQ()` which:
1. Dials AMQP
2. Opens one channel
3. Declares the `trip` exchange and all queues + bindings (idempotent — safe to call on reconnect)

### Routing keys

```
trip.event.created               — trip created, start driver search
trip.event.driver_assigned       — a driver accepted, rider is notified
trip.event.no_drivers_found      — no matching drivers online
trip.event.driver_not_interested — driver declined, retry search
trip.event.cancelled             — rider cancelled trip

driver.cmd.trip_request   — ask a specific driver to accept a trip
driver.cmd.trip_accept    — driver accepted
driver.cmd.trip_decline   — driver declined
driver.cmd.trip_cancelled — tell a specific driver the trip was cancelled
driver.cmd.location       — driver location update (not fully implemented)
driver.cmd.register       — driver registered (WebSocket → browser only)

payment.cmd.create_session       — tell payment-service to create Stripe session
payment.event.session_created    — Stripe session URL ready
payment.event.success            — Stripe webhook confirmed payment
payment.event.failed             — payment failed
payment.event.cancelled          — payment cancelled
```

### Queue bindings

| Queue | Bound routing key(s) | Consumer |
|---|---|---|
| `find_available_drivers` | `trip.event.created`, `trip.event.driver_not_interested` | driver-service |
| `notify_trip_created` | `trip.event.created` | api-gateway → rider WebSocket |
| `trip_cancelled_driver` | `trip.event.cancelled` | driver-service |
| `driver_cmd_trip_cancelled` | `driver.cmd.trip_cancelled` | api-gateway → driver WebSocket |
| `driver_cmd_trip_request` | `driver.cmd.trip_request` | api-gateway → driver WebSocket |
| `driver_cmd_trip_response` | `driver.cmd.trip_accept`, `driver.cmd.trip_decline` | trip-service |
| `notify_no_drivers_found` | `trip.event.no_drivers_found` | api-gateway → rider WebSocket |
| `notify_driver_assigned` | `trip.event.driver_assigned` | api-gateway → rider WebSocket |
| `payment_trip_response` | `payment.cmd.create_session` | payment-service |
| `notify_payment_session_created` | `payment.event.session_created` | api-gateway → rider WebSocket |
| `payment_success` | `payment.event.success` | trip-service |
| `notify_payment_success_gateway` | `payment.event.success` | api-gateway → rider WebSocket |
| `dead_letter_queue` | _(manual)_ | — |

All queues are **durable** (`true`). Messages are published as **persistent** (`DeliveryMode: amqp.Persistent`). QoS prefetch is set to `1` per consumer.

---

## 5. End-to-end flows

### Trip preview and fare selection

```
Browser                api-gateway           trip-service          OSRM
  │                        │                      │                  │
  │ POST /trip/preview      │                      │                  │
  │ {userID, pickup, dest}  │                      │                  │
  │───────────────────────►│                      │                  │
  │                        │ gRPC PreviewTrip      │                  │
  │                        │──────────────────────►│                  │
  │                        │                      │ GET /route/v1/.. │
  │                        │                      │─────────────────►│
  │                        │                      │◄─────────────────│
  │                        │                      │ save RiderFares   │
  │                        │◄──────────────────────│                  │
  │◄───────────────────────│                      │                  │
  │ {route, rideFares[]}    │                      │                  │
```

The rider sees the route drawn on the map and a list of car packages with prices. Selecting one stores the `rideFareID`.

---

### Starting a trip (driver matching)

```
Browser          api-gateway       trip-service      RabbitMQ     driver-service
  │                  │                 │                 │              │
  │ POST /trip/start │                 │                 │              │
  │ {rideFareID,     │                 │                 │              │
  │  userID}         │                 │                 │              │
  │─────────────────►│                 │                 │              │
  │                  │ gRPC CreateTrip │                 │              │
  │                  │────────────────►│                 │              │
  │                  │                 │ save Trip(pending) to MongoDB  │
  │                  │                 │ publish trip.event.created     │
  │                  │                 │────────────────►│              │
  │◄─────────────────│                 │                 │              │
  │ {tripID}         │                 │                 │ find_available_drivers
  │                  │                 │                 │──────────────►│
  │  ← WS: trip.event.created          │ notify_trip_created            │
  │◄═══════════════════════════════════│◄────────────────│              │
  │  "Looking for a driver"            │                 │              │
  │                  │                 │                 │ driver_cmd_trip_request
  │                  │                 │                 │──────────────►│
  │                  │  ← WS: driver.cmd.trip_request    │              │
  │                  │◄══════════════════════════════════│◄─────────────│
  │                  │       (to driver's connection)     │              │
```

---

### Driver accepts / declines

```
Driver browser    api-gateway         RabbitMQ        trip-service
     │                │                   │                │
     │ WS send:        │                   │                │
     │ driver.cmd.trip_accept              │                │
     │────────────────►│                   │                │
     │                 │ publish           │                │
     │                 │ driver.cmd.trip_accept             │
     │                 │──────────────────►│                │
     │                 │                   │ driver_cmd_trip_response
     │                 │                   │───────────────►│
     │                 │                   │                │ UpdateTrip(assigned, driver)
     │                 │                   │                │ publish trip.event.driver_assigned
     │                 │                   │◄───────────────│
     │                 │                   │                │ publish payment.cmd.create_session
     │                 │                   │◄───────────────│
     │ ← WS: trip.event.driver_assigned    │                │
  Rider browser◄═══════════════════════════│                │
```

If the driver declines, `trip.event.driver_not_interested` is published, which re-queues the trip in `find_available_drivers` and the driver-service tries the next available driver.

---

### Payment flow

```
payment-service     RabbitMQ      api-gateway      Browser (Rider)     Stripe
      │                │               │                 │                │
      │ payment_trip_response          │                 │                │
      │◄──────────────│               │                 │                │
      │ Create Stripe Checkout Session │                 │                │
      │──────────────────────────────────────────────────────────────────►│
      │◄──────────────────────────────────────────────────────────────────│
      │ publish payment.event.session_created            │                │
      │──────────────►│               │                 │                │
      │               │ notify_payment_session_created   │                │
      │               │──────────────►│                 │                │
      │               │               │ ← WS: payment.event.session_created
      │               │               │────────────────►│                │
      │               │               │                 │ redirect to Stripe
      │               │               │                 │───────────────►│
      │               │               │                 │◄───────────────│
      │               │               │ POST /webhook/stripe             │
      │               │               │◄────────────────────────────────│
      │               │               │ validate signature               │
      │               │               │ publish payment.event.success    │
      │               │──────────────►│ (payment_success + notify_payment_success_gateway)
  trip-service        │               │                 │
  UpdateTrip("paid")  │               │ ← WS: payment.event.success      │
                      │               │────────────────►│                │
```

---

### Trip cancellation

```
Browser (Rider)   api-gateway     RabbitMQ     driver-service    Driver browser
      │                │               │              │                │
      │ POST /trip/cancel              │              │                │
      │ {tripID, userID}               │              │                │
      │───────────────►│               │              │                │
      │                │ publish       │              │                │
      │                │ trip.event.cancelled         │                │
      │                │──────────────►│              │                │
      │                │               │ trip_cancelled_driver         │
      │                │               │─────────────►│                │
      │                │               │              │ lookup pending driver
      │                │               │              │ publish driver.cmd.trip_cancelled
      │                │               │◄─────────────│                │
      │                │               │ driver_cmd_trip_cancelled      │
      │                │──────────────►│              │                │
      │                │ ← WS: driver.cmd.trip_cancelled               │
      │                │────────────────────────────────────────────── ►│
      │                │               │              │  "Trip cancelled" panel
```

---

## 6. gRPC contracts

### TripService (`proto/trip.proto`)

```protobuf
service TripService {
  rpc PreviewTrip (PreviewTripRequest) returns (PreviewTripResponse);
  rpc CreateTrip  (CreateTripRequest)  returns (CreateTripResponse);
}
```

| RPC | Request fields | Response fields |
|---|---|---|
| `PreviewTrip` | `userID`, `startLocation`, `endLocation` | `tripID`, `route`, `rideFares[]` |
| `CreateTrip` | `rideFareID`, `userID` | `tripID`, `trip` |

### DriverService (`proto/driver.proto`)

```protobuf
service DriverService {
  rpc RegisterDriver   (RegisterDriverRequest) returns (RegisterDriverResponse);
  rpc UnRegisterDriver (RegisterDriverRequest) returns (RegisterDriverResponse);
}
```

Both RPCs take `driverID` + `packageSlug` and return a `Driver` object.

---

## 7. HTTP API

All endpoints are on the API Gateway at `:8081`.

### `POST /trip/preview`

Calculate a route and return fare options.

**Request body:**
```json
{
  "userID": "user-123",
  "pickup":      { "latitude": 49.8397, "longitude": 24.0297 },
  "destination": { "latitude": 49.8420, "longitude": 24.0450 }
}
```

**Response `201`:**
```json
{
  "data": {
    "route": { "geometry": [...], "distance": 2.4, "duration": 7.1 },
    "rideFares": [
      { "id": "...", "packageSlug": "sedan", "totalPriceInCents": 412.5 }
    ]
  }
}
```

---

### `POST /trip/start`

Create a trip and begin driver search. Triggers the entire async matching flow.

**Request body:**
```json
{ "rideFareID": "...", "userID": "user-123" }
```

**Response `201`:**
```json
{ "data": { "tripID": "...", "trip": { ... } } }
```

---

### `POST /trip/cancel`

Cancel an in-progress trip request and notify the pending driver.

**Request body:**
```json
{ "tripID": "...", "userID": "user-123" }
```

**Response `200` (empty body)**

---

### `POST /webhook/stripe`

Stripe webhook endpoint. Validates `Stripe-Signature` header. On `checkout.session.completed` publishes `payment.event.success`.

---

## 8. WebSocket protocol

Connections are at `ws://localhost:8081/ws/riders?userID=<id>` and `ws://localhost:8081/ws/drivers?userID=<id>&packageSlug=<slug>`.

All frames are JSON: `{ "type": "<routing-key>", "data": <payload> }`.

### Server → client messages (both rider and driver)

| `type` | `data` | Who receives it |
|---|---|---|
| `trip.event.created` | `Trip` object | Rider |
| `trip.event.no_drivers_found` | `null` | Rider |
| `trip.event.driver_assigned` | `Trip` (with driver) | Rider |
| `driver.cmd.trip_request` | `Trip` object | Driver |
| `driver.cmd.trip_cancelled` | `null` | Driver |
| `driver.cmd.register` | `Driver` object | Driver |
| `payment.event.session_created` | `{ tripID, sessionID, amount, currency }` | Rider |
| `payment.event.success` | `{ tripID, userID, driverID }` | Rider |

### Client → server messages (driver only)

| `type` | `data` |
|---|---|
| `driver.cmd.trip_accept` | `{ tripID, riderID, driver }` |
| `driver.cmd.trip_decline` | `{ tripID, riderID, driver }` |
| `driver.cmd.location` | location data (handled but not fully forwarded) |

---

## 9. Data models (MongoDB)

Database: `ride-sharing`  
Collections: `trips`, `ride_fares`

### Trip document

```json
{
  "_id": ObjectId,
  "userID": "string",
  "status": "pending | assigned | paid",
  "driver": { "id": "", "name": "", "profilePicture": "", "carPlate": "" },
  "riderFare": {
    "_id": ObjectId,
    "userID": "string",
    "packageSlug": "sedan | suv | van | luxury",
    "totalPriceInCents": 412.5,
    "route": { ... }
  }
}
```

### RiderFare document

```json
{
  "_id": ObjectId,
  "userID": "string",
  "packageSlug": "sedan",
  "totalPriceInCents": 412.5,
  "route": {
    "routes": [{ "distance": 2.4, "duration": 7.1, "geometry": { ... } }]
  }
}
```

---

## 10. Shared packages

**`shared/messaging/`**
- `rabbitmq.go` — `RabbitMQ` struct: dial, reconnect, declare exchange + all queues, `PublishMessage`, `ConsumeMessages`
- `connection_manager.go` — `ConnectionManager`: thread-safe map of `userID → *websocket.Conn`; `Add`, `Remove`, `SendMessage`, `Upgrade`
- `queue_consumer.go` — `QueueConsumer`: wraps `ConsumeMessages` and forwards messages to the correct WebSocket connection by matching `AmqpMessage.OwnerID`
- `events.go` — queue name constants and shared message payload structs

**`shared/contracts/`**
- `amqp.go` — `AmqpMessage` struct + all routing key constants
- `http.go` — `APIResponse` wrapper
- `ws.go` — `WSMessage` struct

**`shared/db/`**
- `mongodb.go` — `NewMongoClient`, `GetDatabase`, collection name constants (`trips`, `ride_fares`)

**`shared/env/`**
- `env.go` — `GetString(key, fallback)` helper

**`shared/retry/`**
- `retry.go` — generic exponential backoff helper

**`shared/proto/`**
- Generated gRPC stubs from `proto/trip.proto` and `proto/driver.proto`

---

## 11. Infrastructure and deployment

### Local development with Tilt

[Tilt](https://tilt.dev) watches source files, recompiles Go binaries, rebuilds Docker images, and redeploys to the local Minikube cluster automatically.

**Compile step (runs on host):**
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/<service> ./services/<service>/...
```

**Live update:** only the compiled binary is synced into the running container — no full image rebuild on every code change.

**Start everything:**
```bash
minikube start
eval $(minikube docker-env)   # point Docker CLI to Minikube's daemon
tilt up
```

Services and ports forwarded to localhost:

| Service | Local port |
|---|---|
| api-gateway | 8081 |
| web (Next.js) | 3000 |
| RabbitMQ AMQP | 5672 |
| RabbitMQ Management UI | 15672 |

### Kubernetes manifests

Located under `infra/development/k8s/`:

| File | What it creates |
|---|---|
| `secrets.yaml` | `rabbitmq-credentials`, `stripe-secrets`, `mongodb-credentials` |
| `app-config.yaml` | `ConfigMap` with shared env vars |
| `rabbitmq-deployment.yaml` | RabbitMQ `Deployment` + `Service` |
| `mongodb-deployment.yaml` | MongoDB `StatefulSet` + `Service` (optional) |
| `api-gateway-deployment.yaml` | API Gateway `Deployment` + `Service` |
| `trip-service-deployment.yaml` | Trip Service `Deployment` + `Service` |
| `driver-service-deployment.yaml` | Driver Service `Deployment` + `Service` |
| `payment-service-deployment.yaml` | Payment Service `Deployment` + `Service` |

**MongoDB connectivity note:** When MongoDB runs on the host (not in-cluster), pods reach it via the Minikube host gateway IP. Find it with:
```bash
minikube ssh "ip route | grep default" | awk '{print $3}'
```
Set the result in `secrets.yaml` → `mongodb-credentials.uri`, e.g. `mongodb://192.168.49.1:27017`.

MongoDB must also be configured to accept external connections:
```bash
sudo sed -i 's/bindIp: 127.0.0.1/bindIp: 0.0.0.0/' /etc/mongod.conf
sudo systemctl restart mongod
```

### Production (GKE)

See the main `README.md` for the step-by-step GKE deployment guide including Docker image builds, Artifact Registry, cluster creation, and HTTPS ingress with Google-managed SSL certificates.

Production manifests live under `infra/production/k8s/`.

---

## 12. Technology reference

### Backend

| Technology | Version | Usage |
|---|---|---|
| Go | 1.23 | All four services |
| gRPC (`google.golang.org/grpc`) | v1.69 | Service-to-service RPC |
| Protocol Buffers | v1.36 | gRPC schema |
| RabbitMQ client (`rabbitmq/amqp091-go`) | v1.10 | Async messaging |
| MongoDB driver (`go.mongodb.org/mongo-driver`) | v1.13 | Trip persistence |
| Gorilla WebSocket (`gorilla/websocket`) | v1.5 | Browser ↔ gateway real-time |
| Stripe Go SDK (`stripe/stripe-go/v81`) | v81 | Payment sessions + webhooks |
| Geohash (`mmcloughlin/geohash`) | v0.10 | Driver location bucketing |
| Google UUID (`google/uuid`) | v1.6 | ID generation |
| OpenTelemetry | v1.34 | Distributed tracing (Jaeger, optional) |

### Frontend

| Technology | Version | Usage |
|---|---|---|
| Next.js | 15.1 | React framework (App Router) |
| React | 19 | UI |
| TypeScript | 5 | Type safety |
| Tailwind CSS | 3.4 | Styling |
| Leaflet + react-leaflet | 1.9 / 5.0 | Interactive maps |
| Stripe.js (`@stripe/stripe-js`) | 5.6 | Payment redirect |
| Radix UI | various | Accessible UI primitives |
| ngeohash / latlon-geohash | 0.6 / 2.0 | Client-side geohash for driver location |
| Lucide React | 0.473 | Icons |

### Infrastructure

| Technology | Usage |
|---|---|
| Kubernetes (Minikube) | Local cluster orchestration |
| Docker | Container images |
| Tilt | Local dev loop (watch → compile → deploy) |
| OSRM (public API) | Driving route calculation |
| Stripe (hosted) | Payment processing |
