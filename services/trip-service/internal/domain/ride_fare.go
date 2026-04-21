package domain

import (
	tripTypes "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RiderFareModel struct {
	ID                primitive.ObjectID
	UserID            string
	PackegeSlug       string // van, luxury, etc.
	TotalPriceInCents float64
	Route             *tripTypes.OsrmAPIResponse
}

func (fare *RiderFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                fare.ID.Hex(),
		UserID:            fare.UserID,
		PackageSlug:       fare.PackegeSlug,
		TotalPriceInCents: fare.TotalPriceInCents,
	}
}

func ToRideFareProto(fares []*RiderFareModel) []*pb.RideFare {
	var protos []*pb.RideFare
	for _, fare := range fares {
		protos = append(protos, fare.ToProto())
	}
	return protos
}
