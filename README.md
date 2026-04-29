# Ride-Sharing Platform

A production-style, event-driven ride-sharing backend built with Go microservices, RabbitMQ, gRPC, MongoDB, and Stripe — with a Next.js frontend and full Kubernetes deployment.

## Overview

Four independent Go services communicate through a RabbitMQ topic exchange and gRPC:

| Service | Responsibility |
|---|---|
| **api-gateway** | HTTP + WebSocket entry point; bridges browser clients to backend services |
| **trip-service** | Trip lifecycle, fare calculation, route planning (OSRM), MongoDB persistence |
| **driver-service** | In-memory driver registry, geohash-based matching, trip dispatch |
| **payment-service** | Stripe Checkout session creation, webhook processing |

The frontend (`web/`) is a Next.js 15 app with real-time WebSocket updates, interactive Leaflet maps, and Stripe.js for payments.

For architecture diagrams, message flow sequences, API contracts, and a full developer reference see **[ARCHITECTURE.md](./ARCHITECTURE.md)**.

## Trip Scheduling Flow

[![](https://mermaid.ink/img/pako:eNqNVt9v2jAQ_lcsP21qGvGjZSEPlSpaTX1YxWDVpAmpMvZBIkicOQ6UVf3fd4mdEgcKzQOK47v7vjt_d-aVcimAhnSW5vC3gJTDXcyWiiWzlOCTMaVjHmcs1eQpB3X49Xb88J1p2LLd4d4vFWdTUJuYw-HmnYo3oM5sH34fs10Cqf7Qb6oRFL-bnZLz5c3NnmRIRgrwteJGJmXOuTa2eyP0aFAPyXIyHtWO5YaxN7-PEoNJpNrM1nOSCw3Y_QuPWLq0nBvWl4h30fIos_Bhg5n6vMIVDTgVLyNN5II4LO9L65A8wrbyJimAyAkjwlbSBHBwENisQ2vl80T4pfezapbGRW1RHckkYaloILMNi9dsvgYXtHUQv2E-lXwF-hCccQ5ZE3sNiwZ0A_O2sjSw6oPTbB_nWbT3TB3dGEiykGrLlABBtCQTNp_H-sdP8sUwK3EmkGcS2-lnAQV8rUtwVCchGSvJIc8tJ2KoMGxDjxSZKIVapZZrpovc9_2j4nF7whGPifvM8jxepp8XkW2SzAQmOVKMZXoyF6_Nwq5bunetkLxp2HfIUQR8JQts5Bqz9DJGx3K1ZtZdHAVpCc9mZStkc3s-0WZtTFukOsGnB5QeE7u6PM4gKScQsozktmF_TmuN1qidUHZJa6q9V04m2RowlrV1SnbQc5GUq33YacFL_R2faHtPzxXt0ZN1O-6iXbRW1Zu4HxeiqjTJivk6ziPTcp-U1aXD-NPgZ46a21KL061Qj6kzc38_facarzCyv1tOdGgVk7OUpCipOSxjZEI9moBKWCzwKn8tQ8yojiCBGQ3xVTC1muEV_4Z2rNByuks5DbUqwKNKFsuIhgu2znFlZo79C1Cb4L36R8rmkoav9IWGvW_-1XVn0O_1-kE3GAyHgUd3-Lnb8fu9frc_xKfbvQ6CN4_-qyJ0_KDX7Q86QTDoDAfD66ve23_1IPGQ?type=png)](https://mermaid.live/edit#pako:eNqNVt9v2jAQ_lcsP21qGvGjZSEPlSpaTX1YxWDVpAmpMvZBIkicOQ6UVf3fd4mdEgcKzQOK47v7vjt_d-aVcimAhnSW5vC3gJTDXcyWiiWzlOCTMaVjHmcs1eQpB3X49Xb88J1p2LLd4d4vFWdTUJuYw-HmnYo3oM5sH34fs10Cqf7Qb6oRFL-bnZLz5c3NnmRIRgrwteJGJmXOuTa2eyP0aFAPyXIyHtWO5YaxN7-PEoNJpNrM1nOSCw3Y_QuPWLq0nBvWl4h30fIos_Bhg5n6vMIVDTgVLyNN5II4LO9L65A8wrbyJimAyAkjwlbSBHBwENisQ2vl80T4pfezapbGRW1RHckkYaloILMNi9dsvgYXtHUQv2E-lXwF-hCccQ5ZE3sNiwZ0A_O2sjSw6oPTbB_nWbT3TB3dGEiykGrLlABBtCQTNp_H-sdP8sUwK3EmkGcS2-lnAQV8rUtwVCchGSvJIc8tJ2KoMGxDjxSZKIVapZZrpovc9_2j4nF7whGPifvM8jxepp8XkW2SzAQmOVKMZXoyF6_Nwq5bunetkLxp2HfIUQR8JQts5Bqz9DJGx3K1ZtZdHAVpCc9mZStkc3s-0WZtTFukOsGnB5QeE7u6PM4gKScQsozktmF_TmuN1qidUHZJa6q9V04m2RowlrV1SnbQc5GUq33YacFL_R2faHtPzxXt0ZN1O-6iXbRW1Zu4HxeiqjTJivk6ziPTcp-U1aXD-NPgZ46a21KL061Qj6kzc38_facarzCyv1tOdGgVk7OUpCipOSxjZEI9moBKWCzwKn8tQ8yojiCBGQ3xVTC1muEV_4Z2rNByuks5DbUqwKNKFsuIhgu2znFlZo79C1Cb4L36R8rmkoav9IWGvW_-1XVn0O_1-kE3GAyHgUd3-Lnb8fu9frc_xKfbvQ6CN4_-qyJ0_KDX7Q86QTDoDAfD66ve23_1IPGQ)

## Tech stack

**Backend:** Go 1.23 · gRPC / protobuf · RabbitMQ (AMQP 0-9-1) · MongoDB · Stripe Go SDK · Gorilla WebSocket · OpenTelemetry / Jaeger (optional)

**Frontend:** Next.js 15 · React 19 · TypeScript · Tailwind CSS · Leaflet / react-leaflet · Stripe.js

**Infrastructure:** Kubernetes · Minikube · Docker · Tilt

## Requirements

- Go 1.23+
- Docker
- Minikube (or any local Kubernetes cluster)
- Tilt
- kubectl
- MongoDB (local or in-cluster)

## Installation

### macOS

```bash
brew install go
brew install minikube
brew install tilt-dev/tap/tilt
```

Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/).

### Linux (Ubuntu / Debian)

```bash
# Go
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc

# Minikube
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# Tilt
curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash
```

Install [Docker](https://docs.docker.com/engine/install/ubuntu/) and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/).

**MongoDB — allow connections from Minikube pods:**
```bash
sudo sed -i 's/bindIp: 127.0.0.1/bindIp: 0.0.0.0/' /etc/mongod.conf
sudo systemctl restart mongod

# Find the host IP as seen from inside Minikube, then set it in infra/development/k8s/secrets.yaml
minikube ssh "ip route | grep default" | awk '{print $3}'
# e.g. mongodb://192.168.49.1:27017
```

### Windows (WSL)

```bash
# Get the Go binary
wget https://dl.google.com/go/go1.23.0.linux-amd64.tar.gz
sudo tar -xvf go1.23.0.linux-amd64.tar.gz
sudo mv go /usr/local

# Add to ~/.bashrc
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

go version
```

Install [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/), [Minikube](https://minikube.sigs.k8s.io/docs/), [Tilt](https://tilt.dev/), and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/).

## Running locally

```bash
minikube start
eval $(minikube docker-env)
tilt up
```

| URL | What |
|---|---|
| http://localhost:3000 | Frontend |
| http://localhost:8081 | API Gateway |
| http://localhost:10350 | Tilt UI |
| http://localhost:15672 | RabbitMQ Management (guest/guest) |

## Stripe webhooks

To test the full payment flow locally, forward Stripe events to the gateway using the [Stripe CLI](https://stripe.com/docs/stripe-cli):

```bash
stripe listen --forward-to localhost:8081/webhook/stripe
```

Copy the printed signing secret into `infra/development/k8s/secrets.yaml` under `stripe-webhook-key`.

## Monitoring

```bash
kubectl get pods
minikube dashboard

# Tail logs for a specific service
kubectl logs -f deployment/api-gateway
kubectl logs -f deployment/trip-service
kubectl logs -f deployment/driver-service
kubectl logs -f deployment/payment-service
```

## Deployment (Google Cloud / GKE)

### 0. Set environment variables

```bash
REGION=europe-west1       # change to your region
PROJECT_ID=<your-project>
```

### 1. Create a secrets file for production

Copy `infra/development/k8s/secrets.yaml` to `infra/production/k8s/secrets.yaml` and fill in production values.

### 2. Build and push Docker images

```bash
docker build -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/ride-sharing/api-gateway:latest \
  --platform linux/amd64 -f infra/production/docker/api-gateway.Dockerfile .

docker build -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/ride-sharing/trip-service:latest \
  --platform linux/amd64 -f infra/production/docker/trip-service.Dockerfile .

docker build -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/ride-sharing/driver-service:latest \
  --platform linux/amd64 -f infra/production/docker/driver-service.Dockerfile .

docker build -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/ride-sharing/payment-service:latest \
  --platform linux/amd64 -f infra/production/docker/payment-service.Dockerfile .
```

Create an Artifact Registry repository in Google Cloud, then push:

```bash
gcloud auth configure-docker ${REGION}-docker.pkg.dev
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/ride-sharing/api-gateway:latest
# repeat for each image
```

### 3. Create a GKE cluster

Create a cluster via the GCP console or `gcloud container clusters create`.

### 4. Apply Kubernetes manifests

```bash
gcloud container clusters get-credentials ride-sharing --region ${REGION} --project ${PROJECT_ID}

kubectl apply -f infra/production/k8s/app-config.yaml
kubectl apply -f infra/production/k8s/secrets.yaml
kubectl apply -f infra/production/k8s/rabbitmq-deployment.yaml

# Wait for RabbitMQ to be Ready, then:
kubectl apply -f infra/production/k8s/api-gateway-deployment.yaml
kubectl apply -f infra/production/k8s/driver-service-deployment.yaml
kubectl apply -f infra/production/k8s/trip-service-deployment.yaml
kubectl apply -f infra/production/k8s/payment-service-deployment.yaml
```

To redeploy or force a pod restart:
```bash
kubectl rollout restart deployment
```

### 5. Get the external IP

```bash
kubectl get services
```

Switch back to local development:
```bash
kubectl config use-context minikube
```

### Adding HTTPS

1. Reserve a static IP in GCP: **VPC Network → External IP addresses → Reserve Static Address**. Name it `api-gateway-ip`.

```bash
gcloud compute addresses list
```

2. Apply the ingress and update the gateway service type to `ClusterIP`:

```bash
kubectl apply -f infra/production/k8s/api-gateway-ingress.yaml
kubectl apply -f infra/production/k8s/api-gateway-deployment.yaml
```

3. Wait for the Google-managed SSL certificate to be provisioned:

```bash
kubectl describe managedcertificate api-gateway-cert
```

Once status shows `Active`, the API is available at `https://<your-domain>`. Use a proper domain name in production rather than a bare IP.
