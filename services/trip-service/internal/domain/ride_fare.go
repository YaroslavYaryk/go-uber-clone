package domain

import (
	tripTypes "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RiderFareModel struct {
	ID                primitive.ObjectID         `bson:"_id,omitempty"`
	UserID            string                     `bson:"userId"`
	PackegeSlug       string                     `bson:"packegeSlug"`
	TotalPriceInCents float64                    `bson:"totalPriceInCents"`
	Route             *tripTypes.OsrmAPIResponse `bson:"route"`
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
