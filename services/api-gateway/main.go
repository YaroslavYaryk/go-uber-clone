package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"ride-sharing/shared/messaging"
	"syscall"
	"time"

	"ride-sharing/shared/env"
)

var (
	httpAddr    = env.GetString("HTTP_ADDR", ":8081")
	rabbitmqURI = env.GetString("RABBITMQ_URI", "amqp://guest:guest@localhost:5672/")
)

func main() {
	log.Println("Starting API Gateway")

	mux := http.NewServeMux()

	//RabbitMQ connection
	rabbitmq, err := messaging.NewRabbitMQ(rabbitmqURI)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmq.Close()

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	startConsumers(appCtx, rabbitmq)

	mux.HandleFunc("POST /trip/preview", enableCORS(handleTripPreview))
	mux.HandleFunc("POST /trip/start", enableCORS(handleTripStart))
	mux.HandleFunc("POST /trip/cancel", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		handleTripCancel(w, r, rabbitmq)
	}))
	mux.HandleFunc("/ws/drivers", func(w http.ResponseWriter, r *http.Request) {
		handleDriverWebSocket(w, r, rabbitmq)
	})
	mux.HandleFunc("/ws/riders", handleRiderWebSocket)

	mux.HandleFunc("/webhook/stripe", func(w http.ResponseWriter, r *http.Request) {
		handleStripeWebhook(w, r, rabbitmq)
	})

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Server listening on %s", httpAddr)
		serverErrors <- server.ListenAndServe()
	}()

	log.Println("Connected to RabbitMQ")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Println("Error starting server:", err)
	case sig := <-shutdown:
		log.Println("Shutting down server gracefully:", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Println("Error during shutdown:", err)
			err = server.Close()
			if err != nil {
				return
			}
		}
	}
}
