package repository

import (
	"context"
	"fmt"
	"ride-sharing/services/trip-service/internal/domain"
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
