package main

import (
	"ride-sharing/services/api-gateway/internal/validator"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"
)

type previewTripRequest struct {
	UserID      string           `json:"userID"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}

func (p *previewTripRequest) toProto() *pb.PreviewTripRequest {
	return &pb.PreviewTripRequest{
		UserID: p.UserID,
		StartLocation: &pb.Coordinate{
			Latitude:  *p.Pickup.Latitude,
			Longitude: *p.Pickup.Longitude,
		},
		EndLocation: &pb.Coordinate{
			Latitude:  *p.Destination.Latitude,
			Longitude: *p.Destination.Longitude,
		},
	}
}

type startTripRequest struct {
	RideFareID string `json:"rideFareID"`
	UserID     string `json:"userID"`
}

func (c *startTripRequest) toProto() *pb.CreateTripRequest {
	return &pb.CreateTripRequest{
		RideFareID: c.RideFareID,
		UserID:     c.UserID,
	}
}

func ValidateTripPreview(v *validator.Validator, previewTrip *previewTripRequest) {
	// Перевіряємо, чи поле не nil (якщо ми використовуємо вказівники для обов'язкових полів)
	v.Check(previewTrip.UserID != "", "userID", "must be provided")
	v.Check(previewTrip.Pickup.Latitude != nil && previewTrip.Pickup.Longitude != nil, "pickup latitude and longitude", "must not be empty")
	v.Check(previewTrip.Destination.Latitude != nil && previewTrip.Destination.Longitude != nil, "destination latitude and longitude", "must not be empty")

	//v.Check(*previewTrip.Pickup.Latitude <= 90 && *previewTrip.Pickup.Latitude >= -90, "pickup latitude", "must be withing -90; 90 range")
	//v.Check(*previewTrip.Pickup.Longitude <= 180 && *previewTrip.Pickup.Longitude >= -180, "pickup longitude", "must be withing -180 180 range")
	//
	//v.Check(*previewTrip.Destination.Latitude <= 90 && *previewTrip.Destination.Latitude >= -90, "destination latitude", "must be withing -90; 90 range")
	//v.Check(*previewTrip.Destination.Longitude <= 180 && *previewTrip.Destination.Longitude >= -180, "destination longitude", "must be withing -180 180 range")
}
