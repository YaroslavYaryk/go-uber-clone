package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"ride-sharing/services/api-gateway/internal/grpc_clients"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"
	"ride-sharing/shared/proto/driver"
	"time"

	"github.com/gorilla/websocket"
)

var (
	connManager = messaging.NewConnectionManager()
)

func startConsumers(ctx context.Context, rb *messaging.RabbitMQ) {
	for _, queue := range []string{messaging.NotifyTripCreatedQueue, messaging.DriverCmdTripRequestQueue, messaging.DriverCmdTripCancelledQueue, messaging.NotifyNoDriversFoundQueue, messaging.NotifyDriverAssignedQueue, messaging.NotifyPaymentSessionCreatedQueue, messaging.NotifyPaymentSuccessGatewayQueue} {
		consumer := messaging.NewQueueConsumer(rb, connManager, queue)
		if err := consumer.Start(ctx); err != nil {
			log.Fatalf("failed to start consumer for %s: %v", queue, err)
		}
	}
}

func handleRiderWebSocket(w http.ResponseWriter, r *http.Request) {

	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("Error upgrading to websocket, %v\n", err)
		return
	}

	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("User ID is missing")
		return
	}

	connManager.Add(userID, conn)
	defer connManager.Remove(userID)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message: ", err)
			break
		}
		log.Println("Received message: ", string(message))
	}

}

func handleDriverWebSocket(w http.ResponseWriter, r *http.Request, rb *messaging.RabbitMQ) {

	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("Error upgrading to websocket, %v\n", err)
		return
	}

	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("User ID is missing")
		return
	}

	packageSlug := r.URL.Query().Get("packageSlug")
	if packageSlug == "" {
		log.Println("packageSlug is missing")
		return
	}

	connManager.Add(userID, conn)

	driverService, err := grpc_clients.NewDriverServiceClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Closing connections
	defer func() {
		connManager.Remove(userID)

		unregCtx, unregCancel := context.WithTimeout(context.Background(), time.Second*5)
		defer unregCancel()

		_, err2 := driverService.Client.UnRegisterDriver(unregCtx, &driver.RegisterDriverRequest{
			DriverID:    userID,
			PackageSlug: packageSlug,
		})
		if err2 != nil {
			log.Printf("Error unregistering driver: %v", err2)
		}

		driverService.Close()

		log.Println("Driver unregistered: ", userID)
	}()

	driverData, err := driverService.Client.RegisterDriver(ctx, &driver.RegisterDriverRequest{
		DriverID:    userID,
		PackageSlug: packageSlug,
	})
	if err != nil {
		log.Printf("Error registering driver: %v", err)
		return
	}

	msg := contracts.WSMessage{
		Type: "driver.cmd.register",
		Data: driverData.Driver,
	}

	if err3 := connManager.SendMessage(userID, msg); err3 != nil {
		log.Println("Error writing message: ", err3)
		return
	}

	for {
		_, message, err4 := conn.ReadMessage()
		if err4 != nil {
			log.Println("Error reading message: ", err4)
			break
		}

		type DriverMessage struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		var driverMsg DriverMessage
		if err5 := json.Unmarshal(message, &driverMsg); err5 != nil {
			log.Println("Error unmarshalling message: ", err5)
			continue
		}

		// handle the different message type

		switch driverMsg.Type {
		case contracts.DriverCmdLocation:
			//handle driver location update
			continue
		case contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline:
			// forward the message to rabbitmq
			if err = rb.PublishMessage(ctx, driverMsg.Type, contracts.AmqpMessage{
				OwnerID: userID,
				Data:    driverMsg.Data,
			}); err != nil {
				log.Println("Error publishing message to rbmq: ", err)
			}
		default:
			log.Println("Unsupported message type: ", driverMsg.Type)
		}

	}

}
