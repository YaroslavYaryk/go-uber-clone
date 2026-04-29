package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"
	pbd "ride-sharing/shared/proto/driver"

	amqp "github.com/rabbitmq/amqp091-go"
)

type driverConsumer struct {
	rabbitMQ *messaging.RabbitMQ
	service  domain.TripService
}

func NewDriverConsumer(rabbitMQ *messaging.RabbitMQ, service domain.TripService) *driverConsumer {
	return &driverConsumer{rabbitMQ: rabbitMQ, service: service}
}

func (c *driverConsumer) Listen() error {
	return c.rabbitMQ.ConsumeMessages(messaging.DriverCmdTripResponseQueue, func(ctx context.Context, msg amqp.Delivery) error {

		var message contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &message); err != nil {
			log.Printf("failed to unmarshal AmqpMessage: %v", err)
			return err
		}

		var payload messaging.DriverTripResponseData
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("failed to unmarshal AmqpMessage2: %v", err)
			return err
		}

		switch msg.RoutingKey {
		case contracts.DriverCmdTripAccept:
			if err := c.handleTripAccepted(ctx, payload.TripID, payload.Driver); err != nil {
				log.Printf("failed to handle trip accepted: %v", err)
				return err
			}
		case contracts.DriverCmdTripDecline:
			if err := c.handleTripDeclined(ctx, payload.TripID, payload.RiderID); err != nil {
				log.Printf("failed to handle trip declined: %v", err)
				return err
			}
		default:
			log.Printf("UNknow trip events: %s", msg.RoutingKey)
		}

		return nil
	})
}

func (c *driverConsumer) handleTripAccepted(ctx context.Context, tripID string, driver *pbd.Driver) error {

	//fetch the trip from the db
	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}
	if trip == nil {
		return fmt.Errorf("trip not found with id %s", tripID)
	}

	//update the trip
	if err := c.service.UpdateTrip(ctx, tripID, "accepted", driver); err != nil {
		log.Printf("failed to update trip accepted: %v", err)
		return err
	}

	trip, err = c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	// driver has been assigned to this trip
	marshalledTrip, err := json.Marshal(trip)
	if err != nil {
		return err
	}

	// notify the rider that driver has been assigned
	if err = c.rabbitMQ.PublishMessage(ctx, contracts.TripEventDriverAssigned, contracts.AmqpMessage{
		OwnerID: trip.UserID,
		Data:    marshalledTrip,
	}); err != nil {
		log.Printf("failed to publish suitable drivers: %v", err)
		return err
	}

	marshalledPayload, err := json.Marshal(messaging.PaymentTripResponseData{
		TripID:   tripID,
		UserID:   trip.UserID,
		DriverID: driver.Id,
		Amount:   trip.RiderFare.TotalPriceInCents,
		Currency: "USD",
	})

	if err := c.rabbitMQ.PublishMessage(ctx, contracts.PaymentCmdCreateSession,
		contracts.AmqpMessage{
			OwnerID: trip.UserID,
			Data:    marshalledPayload,
		},
	); err != nil {
		return err
	}

	return nil
}

func (c *driverConsumer) handleTripDeclined(ctx context.Context, tripID string, riderID string) error {

	//fetch the trip from the db
	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	newPayload := messaging.TripEventData{
		Trip: trip.ToProto(),
	}

	// driver has been assigned to this trip
	marshalledTrip, err := json.Marshal(newPayload)
	if err != nil {
		return err
	}

	// notify the rider that driver has been assigned
	if err = c.rabbitMQ.PublishMessage(ctx, contracts.TripEventDriverNotInterested, contracts.AmqpMessage{
		OwnerID: riderID,
		Data:    marshalledTrip,
	}); err != nil {
		log.Printf("failed to publish suitable drivers: %v", err)
		return err
	}

	return nil
}
