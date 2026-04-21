package domain

import (
	"context"
	tripTypes "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TripModel struct {
	ID        primitive.ObjectID
	UserID    string
	Status    string
	RiderFare *RiderFareModel
	Driver    *pb.TripDriver
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, fare *RiderFareModel) error

	GetRideFareByID(ctx context.Context, fareID string) (*RiderFareModel, error)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RiderFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup *types.Coordinate, destination *types.Coordinate) (*tripTypes.OsrmAPIResponse, error)
	EstimatePachegesPriceWithRoute(ctx context.Context, route *tripTypes.OsrmAPIResponse) []*RiderFareModel
	GenerateTripFares(ctx context.Context, fares []*RiderFareModel, userID string, route *tripTypes.OsrmAPIResponse) ([]*RiderFareModel, error)

	GetAndValidateFare(ctx context.Context, fareID, userId string) (*RiderFareModel, error)
}
