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
	"ride-sharing/shared/util"
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
