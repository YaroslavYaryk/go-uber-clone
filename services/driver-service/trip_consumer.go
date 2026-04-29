package main

import (
	"context"
	"encoding/json"
	"log"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type tripConsumer struct {
	rabbitMQ *messaging.RabbitMQ
	service  *Service
}

func NewTripConsumer(rabbitMQ *messaging.RabbitMQ, service *Service) *tripConsumer {
	return &tripConsumer{rabbitMQ: rabbitMQ, service: service}
}

func (c *tripConsumer) Listen() error {
	return c.rabbitMQ.ConsumeMessages(messaging.FindAvailableDriversQueue, func(ctx context.Context, msg amqp.Delivery) error {

		var tripEvent contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &tripEvent); err != nil {
			log.Printf("failed to unmarshal AmqpMessage: %v", err)
			return err
		}

		var payload messaging.TripEventData
		if err := json.Unmarshal(tripEvent.Data, &payload); err != nil {
			log.Printf("failed to unmarshal AmqpMessage2: %v", err)
			return err
		}

		switch msg.RoutingKey {
		case contracts.TripEventCreated, contracts.TripEventDriverNotInterested:
			return c.handleFindAndNotifyDriver(ctx, payload)
		}

		log.Printf("UNknow trip events: %s", msg.RoutingKey)

		return nil
	})
}

func (c *tripConsumer) handleFindAndNotifyDriver(ctx context.Context, payload messaging.TripEventData) error {

	suitableIDs := c.service.FindAvailableDrivers(payload.Trip.SelectedFare.PackageSlug)

	log.Printf("suitable Drivers: %v", suitableIDs)

	if len(suitableIDs) == 0 {
		// notify rider that ni drivers are available
		if err := c.rabbitMQ.PublishMessage(ctx, contracts.TripEventNoDriversFound, contracts.AmqpMessage{
			OwnerID: payload.Trip.UserID,
		}); err != nil {
			log.Printf("failed to publish suitable drivers: %v", err)
		}

		return nil
	}

	suitableDriverID := suitableIDs[0]

	log.Printf("Storing pending trip: tripID=%s -> driverID=%s", payload.Trip.Id, suitableDriverID)
	c.service.SetPendingDriver(payload.Trip.Id, suitableDriverID)

	marshalledEvent, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal AmqpMessage: %v", err)
		return err
	}

	// notify Driver for a pottential trip
	if err := c.rabbitMQ.PublishMessage(ctx, contracts.DriverCmdTripRequest, contracts.AmqpMessage{
		OwnerID: suitableDriverID,
		Data:    marshalledEvent,
	}); err != nil {
		log.Printf("failed to publish suitable drivers: %v", err)
		return err
	}

	return nil
}

func (c *tripConsumer) ListenForCancellations() error {
	log.Printf("Starting cancellation consumer on queue: %s", messaging.TripCancelledDriverQueue)
	return c.rabbitMQ.ConsumeMessages(messaging.TripCancelledDriverQueue, func(ctx context.Context, msg amqp.Delivery) error {
		log.Printf("Received cancellation message: %s", string(msg.Body))

		var tripEvent contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &tripEvent); err != nil {
			log.Printf("failed to unmarshal cancellation message: %v", err)
			return err
		}

		if tripEvent.Data == nil {
			log.Printf("cancellation message has no data")
			return nil
		}

		var payload messaging.TripCancelledData
		if err := json.Unmarshal(tripEvent.Data, &payload); err != nil {
			log.Printf("failed to unmarshal TripCancelledData: %v", err)
			return err
		}

		log.Printf("Trip cancelled: tripID=%s, looking up pending driver", payload.TripID)

		driverID := c.service.GetAndRemovePendingDriver(payload.TripID)
		if driverID == "" {
			log.Printf("no pending driver found for cancelled trip %s (map size: %d)", payload.TripID, len(c.service.pendingTrips))
			return nil
		}

		log.Printf("Notifying driver %s of cancellation for trip %s", driverID, payload.TripID)

		if err := c.rabbitMQ.PublishMessage(ctx, contracts.DriverCmdTripCancelled, contracts.AmqpMessage{
			OwnerID: driverID,
		}); err != nil {
			log.Printf("failed to publish trip cancelled to driver: %v", err)
			return err
		}

		return nil
	})
}
