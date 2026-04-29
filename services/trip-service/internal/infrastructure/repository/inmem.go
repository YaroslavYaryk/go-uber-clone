package repository

import (
	"context"
	"fmt"
	"ride-sharing/services/trip-service/internal/domain"
	pbd "ride-sharing/shared/proto/driver"
	pb "ride-sharing/shared/proto/trip"
	"sync"
)

type InMemRepository struct {
	trips     map[string]*domain.TripModel
	redeFares map[string]*domain.RiderFareModel
	sync.RWMutex
}

func NewInMemRepository() *InMemRepository {
	return &InMemRepository{
		trips:     make(map[string]*domain.TripModel),
		redeFares: make(map[string]*domain.RiderFareModel),
	}
}

func (repo *InMemRepository) CreateTrip(ctx context.Context, trip *domain.TripModel) (*domain.TripModel, error) {

	repo.Lock()
	defer repo.Unlock()

	repo.trips[trip.ID.Hex()] = trip
	return trip, nil
}

func (r *InMemRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	trip, ok := r.trips[id]
	if !ok {
		return nil, nil
	}
	return trip, nil
}

func (r *InMemRepository) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	trip, ok := r.trips[tripID]
	if !ok {
		return fmt.Errorf("trip not found with ID: %s", tripID)
	}

	trip.Status = status

	if driver != nil {
		trip.Driver = &pb.TripDriver{
			Id:             driver.Id,
			Name:           driver.Name,
			CarPlate:       driver.CarPlate,
			ProfilePicture: driver.ProfilePicture,
		}
	}
	return nil
}

func (repo *InMemRepository) SaveRideFare(ctx context.Context, fare *domain.RiderFareModel) error {

	repo.Lock()
	defer repo.Unlock()

	repo.redeFares[fare.ID.Hex()] = fare

	return nil

}

func (repo *InMemRepository) GetRideFareByID(ctx context.Context, fareID string) (*domain.RiderFareModel, error) {
	repo.RLock()
	defer repo.RUnlock()

	fare, exits := repo.redeFares[fareID]
	if !exits {
		return nil, fmt.Errorf("fare not found")
	}

	return fare, nil

}
