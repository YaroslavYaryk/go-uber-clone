package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ride-sharing/services/trip-service/internal/domain"
	tripTypes "ride-sharing/services/trip-service/pkg/types"
	"ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	repo domain.TripRepository
}

func NewService(repo domain.TripRepository) *Service {
	return &Service{repo: repo}
}

func (service *Service) CreateTrip(ctx context.Context, fare *domain.RiderFareModel) (*domain.TripModel, error) {

	t := domain.TripModel{
		ID:        primitive.NewObjectID(),
		UserID:    fare.UserID,
		RiderFare: fare,
		Status:    "pending",
		Driver:    &trip.TripDriver{},
	}

	return service.repo.CreateTrip(ctx, &t)
}

func (service *Service) GetRoute(ctx context.Context, pickup *types.Coordinate, destination *types.Coordinate) (*tripTypes.OsrmAPIResponse, error) {

	url := fmt.Sprintf("http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		*pickup.Longitude, *pickup.Latitude,
		*destination.Longitude, *destination.Latitude,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch route from osrm API: %v", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var routeResponse tripTypes.OsrmAPIResponse
	if err := json.Unmarshal(body, &routeResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return &routeResponse, nil
}

func (servidce *Service) EstimatePachegesPriceWithRoute(ctx context.Context, route *tripTypes.OsrmAPIResponse) []*domain.RiderFareModel {
	baseFares := getBaseFares()
	estimatedFares := make([]*domain.RiderFareModel, len(baseFares))

	for i, fare := range baseFares {
		estimatedFares[i] = estimateFareRoute(fare, route)
	}

	return estimatedFares
}

func (servidce *Service) GenerateTripFares(ctx context.Context, rideFares []*domain.RiderFareModel, userID string, route *tripTypes.OsrmAPIResponse) ([]*domain.RiderFareModel, error) {
	fares := make([]*domain.RiderFareModel, len(rideFares))

	for i, fare := range rideFares {
		id := primitive.NewObjectID()
		fare := &domain.RiderFareModel{
			TotalPriceInCents: fare.TotalPriceInCents,
			PackegeSlug:       fare.PackegeSlug,
			UserID:            userID,
			ID:                id,
			Route:             route,
		}

		if err := servidce.repo.SaveRideFare(ctx, fare); err != nil {
			return nil, err
		}

		fares[i] = fare
	}

	return fares, nil

}

func (servidce *Service) GetAndValidateFare(ctx context.Context, fareID, userID string) (*domain.RiderFareModel, error) {
	fate, err := servidce.repo.GetRideFareByID(ctx, fareID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fare: %w", err)
	}

	if fate == nil {
		return nil, fmt.Errorf("fare not found")
	}

	if userID != fate.UserID {
		return nil, fmt.Errorf("invalid user id")
	}

	return fate, nil
}

func estimateFareRoute(baseFare *domain.RiderFareModel, route *tripTypes.OsrmAPIResponse) *domain.RiderFareModel {
	pricingCfg := tripTypes.DefaultPricingConfig()
	carPackagePrice := baseFare.TotalPriceInCents

	distanceKm := route.Routes[0].Distance
	durationMinutes := route.Routes[0].Duration

	distanceFare := distanceKm * pricingCfg.PricePerUnitOfDistance
	durationFare := durationMinutes * pricingCfg.PricePerMinute

	totalPrice := carPackagePrice + distanceFare + durationFare

	return &domain.RiderFareModel{
		TotalPriceInCents: totalPrice,
		PackegeSlug:       baseFare.PackegeSlug,
	}

}

func getBaseFares() []*domain.RiderFareModel {
	return []*domain.RiderFareModel{
		{
			PackegeSlug:       "suv",
			TotalPriceInCents: 200,
		},
		{
			PackegeSlug:       "sedan",
			TotalPriceInCents: 350,
		},
		{
			PackegeSlug:       "van",
			TotalPriceInCents: 400,
		},
		{
			PackegeSlug:       "luxury",
			TotalPriceInCents: 1000,
		},
	}
}
