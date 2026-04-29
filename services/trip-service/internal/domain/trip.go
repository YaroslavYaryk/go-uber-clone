package domain

import (
	"context"
	tripTypes "ride-sharing/services/trip-service/pkg/types"
	pbd "ride-sharing/shared/proto/driver"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TripModel struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"userID"`
	Status    string             `bson:"status"`
	RiderFare *RiderFareModel    `bson:"riderFare"`
	Driver    *pb.TripDriver     `bson:"driver"`
}

func (t *TripModel) ToProto() *pb.Trip {
	return &pb.Trip{
		Id:           t.ID.Hex(),
		UserID:       t.UserID,
		SelectedFare: t.RiderFare.ToProto(),
		Status:       t.Status,
		Driver:       t.Driver,
		Route:        t.RiderFare.Route.ToProto(),
	}
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error

	SaveRideFare(ctx context.Context, fare *RiderFareModel) error

	GetRideFareByID(ctx context.Context, fareID string) (*RiderFareModel, error)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RiderFareModel) (*TripModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
	GetRoute(ctx context.Context, pickup *types.Coordinate, destination *types.Coordinate) (*tripTypes.OsrmAPIResponse, error)
	EstimatePachegesPriceWithRoute(ctx context.Context, route *tripTypes.OsrmAPIResponse) []*RiderFareModel
	GenerateTripFares(ctx context.Context, fares []*RiderFareModel, userID string, route *tripTypes.OsrmAPIResponse) ([]*RiderFareModel, error)

	GetAndValidateFare(ctx context.Context, fareID, userId string) (*RiderFareModel, error)
}
