package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"ride-sharing/services/api-gateway/internal/grpc_clients"
	"ride-sharing/services/api-gateway/internal/validator"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
	"ride-sharing/shared/util"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

func handleTripPreview(w http.ResponseWriter, r *http.Request) {

	var reqBody previewTripRequest

	err := util.ReadJSON(w, r, &reqBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(r.Body)

	log.Printf("Trip preview request: %+v", reqBody)

	// Initialize a new Validator.
	v := validator.New()
	// Call the ValidateMovie() function, and if any checks fail, return a response
	// containing the errors.
	if ValidateTripPreview(v, &reqBody); !v.Valid() {
		util.ErrorResponse(w, r, http.StatusBadRequest, v.Errors)
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Println(err)
	}

	defer tripService.Close()

	tripPreview, err := tripService.Client.PreviewTrip(r.Context(), reqBody.toProto())
	if err != nil {
		log.Printf("Error getting trip preview: %v", err)
		http.Error(w, "Failed to preview trip", http.StatusInternalServerError)
		return
	}

	fmt.Println(tripPreview)

	response := contracts.APIResponse{Data: tripPreview}

	err = util.WriteJSON(w, http.StatusCreated, response, nil)
	if err != nil {
		return
	}

}

func handleTripStart(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var reqBody startTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	// Why we need to create a new client for each connection:
	// because if a service is down, we don't want to block the whole application
	// so we create a new client for each connection
	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Fatal(err)
	}

	// Don't forget to close the client to avoid resource leaks!
	defer tripService.Close()

	trip, err := tripService.Client.CreateTrip(ctx, reqBody.toProto())
	if err != nil {
		log.Printf("Failed to start a trip: %v", err)
		http.Error(w, "Failed to start trip", http.StatusInternalServerError)
		return
	}

	response := contracts.APIResponse{Data: trip}

	err = util.WriteJSON(w, http.StatusCreated, response, nil)
	if err != nil {
		return
	}
}

func handleTripCancel(w http.ResponseWriter, r *http.Request, rabbitmq *messaging.RabbitMQ) {
	ctx := r.Context()

	var reqBody struct {
		TripID string `json:"tripID"`
		UserID string `json:"userID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("Trip cancel request: tripID=%s userID=%s", reqBody.TripID, reqBody.UserID)

	if reqBody.TripID == "" || reqBody.UserID == "" {
		http.Error(w, "tripID and userID are required", http.StatusBadRequest)
		return
	}

	payload := messaging.TripCancelledData{
		TripID: reqBody.TripID,
		UserID: reqBody.UserID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to marshal payload", http.StatusInternalServerError)
		return
	}

	if err := rabbitmq.PublishMessage(ctx, contracts.TripEventCancelled, contracts.AmqpMessage{
		OwnerID: reqBody.UserID,
		Data:    payloadBytes,
	}); err != nil {
		log.Printf("failed to publish trip cancelled: %v", err)
		http.Error(w, "failed to cancel trip", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request, rabbitmq *messaging.RabbitMQ) {
	//ctx, span := tracer.Start(r.Context(), "handleStripeWebhook")
	//defer span.End()
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		log.Printf("Webhook key is required")
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body,
		r.Header.Get("Stripe-Signature"),
		webhookKey,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		log.Printf("Error verifying webhook signature: %v", err)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Received Stripe event: %v", event)

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession

		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		payload := messaging.PaymentStatusUpdateData{
			TripID:   session.Metadata["trip_id"],
			UserID:   session.Metadata["user_id"],
			DriverID: session.Metadata["driver_id"],
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling payload: %v", err)
			http.Error(w, "Failed to marshal payload", http.StatusInternalServerError)
			return
		}

		message := contracts.AmqpMessage{
			OwnerID: session.Metadata["user_id"],
			Data:    payloadBytes,
		}

		if err := rabbitmq.PublishMessage(
			ctx,
			contracts.PaymentEventSuccess,
			message,
		); err != nil {
			log.Printf("Error publishing payment event: %v", err)
			http.Error(w, "Failed to publish payment event", http.StatusInternalServerError)
			return
		}
	}
}
