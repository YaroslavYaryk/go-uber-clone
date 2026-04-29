package main

import (
	"math/rand/v2"
	pb "ride-sharing/shared/proto/driver"
	"ride-sharing/shared/util"
	"sync"

	"github.com/mmcloughlin/geohash"
)

type Service struct {
	drivers      []*driverInMap
	mu           *sync.RWMutex
	pendingTrips map[string]string // tripID -> driverID
	pendingMu    sync.RWMutex
}

type driverInMap struct {
	Driver *pb.Driver
}

func NewService() *Service {
	return &Service{
		drivers:      make([]*driverInMap, 0),
		mu:           &sync.RWMutex{},
		pendingTrips: make(map[string]string),
	}
}

func (s *Service) SetPendingDriver(tripID, driverID string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pendingTrips[tripID] = driverID
}

func (s *Service) GetAndRemovePendingDriver(tripID string) string {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	driverID := s.pendingTrips[tripID]
	delete(s.pendingTrips, tripID)
	return driverID
}

func (s *Service) FindAvailableDrivers(packageType string) []string {
	var matchingDrivers []string

	for _, driver := range s.drivers {
		if driver.Driver.PackageSlug == packageType {
			matchingDrivers = append(matchingDrivers, driver.Driver.Id)
		}
	}

	if len(matchingDrivers) == 0 {
		return []string{}
	}

	return matchingDrivers
}

func (s *Service) RegisterDriver(driverId string, packageSlug string) (*pb.Driver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	randomIndex := rand.IntN(len(PredefinedRoutes))
	randomRoute := PredefinedRoutes[randomIndex]

	randomPlate := GenerateRandomPlate()
	randomAvatar := util.GetRandomAvatar(randomIndex)

	// we can ignore this property for now, but it must be sent to the frontend.
	geohash := geohash.Encode(randomRoute[0][0], randomRoute[0][1])

	driver := &pb.Driver{
		Id:             driverId,
		Geohash:        geohash,
		Location:       &pb.Location{Latitude: randomRoute[0][0], Longitude: randomRoute[0][1]},
		Name:           "Lando Norris",
		PackageSlug:    packageSlug,
		ProfilePicture: randomAvatar,
		CarPlate:       randomPlate,
	}

	s.drivers = append(s.drivers, &driverInMap{
		Driver: driver,
	})

	return driver, nil
}

func (s *Service) UnregisterDriver(driverId string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, driver := range s.drivers {
		if driver.Driver.Id == driverId {
			s.drivers = append(s.drivers[:i], s.drivers[i+1:]...)
		}
	}
}
