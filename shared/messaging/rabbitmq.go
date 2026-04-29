package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"ride-sharing/shared/contracts"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
	uri     string
}

const (
	TripExchange = "trip"
)

func NewRabbitMQ(uri string) (*RabbitMQ, error) {
	rmq := &RabbitMQ{uri: uri}
	if err := rmq.reconnect(); err != nil {
		return nil, err
	}
	return rmq, nil
}

func (r *RabbitMQ) reconnect() error {
	con, err := amqp.Dial(r.uri)
	if err != nil {
		return err
	}

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	r.conn = con
	r.Channel = ch

	if err := r.setupExchangesAndQueues(); err != nil {
		r.Close()
		return fmt.Errorf("failed to setup exchanges and queues: %w", err)
	}

	return nil
}

type MessageHandler func(context.Context, amqp.Delivery) error

func (r *RabbitMQ) ConsumeMessages(queueName string, handler MessageHandler) error {

	err1 := r.Channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)

	if err1 != nil {
		return fmt.Errorf("failed to set QoS: %w", err1)
	}

	msgs, err := r.Channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)

	if err != nil {
		return err
	}

	ctx := context.Background()

	go func() {
		for d := range msgs {

			if err := handler(ctx, d); err != nil {
				log.Printf("Failed to handle message: %v", err)

				if nackErr := d.Nack(false, false); nackErr != nil {
					log.Printf("Failed to nack message: %v", nackErr)
				}

				continue
			}

			if ackErr := d.Ack(false); ackErr != nil {
				log.Printf("Failed to ack message: %v", ackErr)
			}

		}
	}()

	return nil
}

func (r *RabbitMQ) PublishMessage(ctx context.Context, routingKey string, message contracts.AmqpMessage) error {
	log.Printf("Publishing message with routing key: %s", routingKey)

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return r.Channel.PublishWithContext(ctx,
		TripExchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         jsonMsg,
			DeliveryMode: amqp.Persistent,
		})
}

func (r *RabbitMQ) setupExchangesAndQueues() error {

	err := r.Channel.ExchangeDeclare(TripExchange, "topic", true, false, false, false, nil)

	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	if err := r.declareAndBindQueue(FindAvailableDriversQueue, []string{contracts.TripEventCreated, contracts.TripEventDriverNotInterested}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyTripCreatedQueue, []string{contracts.TripEventCreated}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(TripCancelledDriverQueue, []string{contracts.TripEventCancelled}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(DriverCmdTripCancelledQueue, []string{contracts.DriverCmdTripCancelled}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(DriverCmdTripRequestQueue, []string{contracts.DriverCmdTripRequest}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(DriverCmdTripResponseQueue, []string{contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyNoDriversFoundQueue, []string{contracts.TripEventNoDriversFound}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyDriverAssignedQueue, []string{contracts.TripEventDriverAssigned}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(PaymentTripResponseQueue, []string{contracts.PaymentCmdCreateSession}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyPaymentSessionCreatedQueue, []string{contracts.PaymentEventSessionCreated}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyPaymentSuccessQueue, []string{contracts.PaymentEventSuccess}, TripExchange); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(NotifyPaymentSuccessGatewayQueue, []string{contracts.PaymentEventSuccess}, TripExchange); err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) declareAndBindQueue(queueName string, messageTypes []string, exchange string) error {

	q, err := r.Channel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return err
	}

	for _, messageType := range messageTypes {
		err = r.Channel.QueueBind(q.Name, messageType, exchange, false, nil)
		if err != nil {
			return fmt.Errorf("failed to bind queue for message %s: %w", messageType, err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.conn == nil {
		return
	}
	err := r.conn.Close()
	if err != nil {
		return
	}

	if r.Channel != nil {
		_ = r.Channel.Close()
	}
}
