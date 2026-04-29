package grpc

import (
	"context"
	"fmt"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/services/trip-service/internal/infrastructure/events"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	pb.UnimplementedTripServiceServer
	service   domain.TripService
	publisher *events.TripEventPublisher
}

func NewGRPCHandler(server *grpc.Server, service domain.TripService, publisher *events.TripEventPublisher) *gRPCHandler {
	handler := &gRPCHandler{
		service:   service,
		publisher: publisher,
	}

	pb.RegisterTripServiceServer(server, handler)
	return handler
}

func (h *gRPCHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {

	fareID := req.GetRideFareID()
	userID := req.GetUserID()

	fare, err := h.service.GetAndValidateFare(ctx, fareID, userID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid fare")
	}

	trip, err := h.service.CreateTrip(ctx, fare)
	if err != nil {
		return nil, err
	}

	if err = h.publisher.PublishTripCreated(ctx, trip); err != nil {
		return nil, fmt.Errorf("failed to publish trip created event: %w", err)
	}

	return &pb.CreateTripResponse{
		TripID: trip.ID.Hex(),
	}, nil
}

func (h *gRPCHandler) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {

	pickup := req.GetStartLocation()
	destination := req.GetEndLocation()

	pickupCoord := types.Coordinate{
		Latitude:  &pickup.Latitude,
		Longitude: &pickup.Longitude,
	}

	destinationCoord := types.Coordinate{
		Latitude:  &destination.Latitude,
		Longitude: &destination.Longitude,
	}

	userID := req.GetUserID()

	route, err := h.service.GetRoute(ctx, &pickupCoord, &destinationCoord)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to get route")
	}

	estimated := h.service.EstimatePachegesPriceWithRoute(ctx, route)

	fares, err := h.service.GenerateTripFares(ctx, estimated, userID, route)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to generate trip fares")
	}

	return &pb.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: domain.ToRideFareProto(fares),
		TripID:    "123",
	}, nil
}
