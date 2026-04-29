package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
	"syscall"

	grpcserver "google.golang.org/grpc"
)

var GrpcAddr = ":9092"

func main() {

	rabbitmqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@localhost:5672/")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	lis, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
	}

	//RabbitMQ connection
	rabbitmq, err := messaging.NewRabbitMQ(rabbitmqURI)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmq.Close()

	log.Println("Connected to RabbitMQ")

	service := NewService()

	grpcServer := grpcserver.NewServer()

	NewGrpcHandler(grpcServer, service)

	consumer := NewTripConsumer(rabbitmq, service)
	go func() {
		if err2 := consumer.Listen(); err2 != nil {
			log.Fatalf("Failed to start consumer: %v", err2)
		}
		if err2 := consumer.ListenForCancellations(); err2 != nil {
			log.Fatalf("Failed to start cancellation consumer: %v", err2)
		}
		log.Println("All consumers started")
	}()

	log.Println("Starting gRPC server...")

	go func() {
		err := grpcServer.Serve(lis)
		if err != nil {
			log.Printf("failed to serve: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gRPC server...")
	grpcServer.GracefulStop()

}
