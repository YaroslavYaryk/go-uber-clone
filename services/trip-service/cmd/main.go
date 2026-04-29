package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"ride-sharing/services/trip-service/internal/infrastructure/events"
	"ride-sharing/services/trip-service/internal/infrastructure/grpc"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
	"ride-sharing/shared/db"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
	"syscall"

	"go.mongodb.org/mongo-driver/mongo"
	grpcserver "google.golang.org/grpc"
)

var GrpcAddr = ":9093"

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
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("listening on %s", GrpcAddr)

	//initialize mongo
	mongoClient, err := db.NewMongoClient(ctx, db.NewMongoDefaultConfig())
	if err != nil {
		log.Fatalf("failed to connect to mongodb: %v", err)
	}

	defer func(mongoClient *mongo.Client, ctx context.Context) {
		err := mongoClient.Disconnect(ctx)
		if err != nil {
			log.Fatalf("failed to disconnect mongo: %v", err)
		}
	}(mongoClient, ctx)

	mongoDb := db.GetDatabase(mongoClient, db.NewMongoDefaultConfig())

	log.Println("Connected to MongoDB", mongoDb)

	mongoDbRepo := repository.NewMongoRepository(mongoDb)
	svc := service.NewService(mongoDbRepo)

	//RabbitMQ connection
	rabbitmq, err := messaging.NewRabbitMQ(rabbitmqURI)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmq.Close()

	log.Println("Connected to RabbitMQ")

	publisher := events.NewTripEventPublisher(rabbitmq)

	//start Driver consumer
	driverConsumer := events.NewDriverConsumer(rabbitmq, svc)
	go func() {
		err := driverConsumer.Listen()
		if err != nil {
			log.Fatalf("failed to listen driver consumer: %v", err)
		}
	}()

	//payment consumer
	paymentConsumer := events.NewPaymentConsumer(rabbitmq, svc)
	go func() {
		err := paymentConsumer.Listen()
		if err != nil {
			log.Fatalf("failed to listen payment consumer: %v", err)
		}
	}()

	grpcServer := grpcserver.NewServer()
	//pb.RegisterTripServiceServer(grpcServer, svc)

	grpc.NewGRPCHandler(grpcServer, svc, publisher)

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
