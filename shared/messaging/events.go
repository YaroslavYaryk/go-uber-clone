package messaging

import (
	pbd "ride-sharing/shared/proto/driver"
	pb "ride-sharing/shared/proto/trip"
)

const (
	FindAvailableDriversQueue        = "find_available_drivers"
	DriverCmdTripRequestQueue        = "driver_cmd_trip_request"
	DriverCmdTripResponseQueue       = "driver_cmd_trip_response"
	DriverCmdTripCancelledQueue      = "driver_cmd_trip_cancelled"
	TripCancelledDriverQueue         = "trip_cancelled_driver"
	NotifyTripCreatedQueue           = "notify_trip_created"
	NotifyNoDriversFoundQueue        = "notify_no_drivers_found"
	NotifyDriverAssignedQueue        = "notify_driver_assigned"
	PaymentTripResponseQueue         = "payment_trip_response"
	NotifyPaymentSessionCreatedQueue = "notify_payment_session_created"
	NotifyPaymentSuccessQueue        = "payment_success"
	NotifyPaymentSuccessGatewayQueue = "notify_payment_success_gateway"
	DeadLetterQueue                  = "dead_letter_queue"
)

type TripEventData struct {
	Trip *pb.Trip `json:"trip"`
}

type DriverTripResponseData struct {
	Driver  *pbd.Driver `json:"driver"`
	TripID  string      `json:"tripID"`
	RiderID string      `json:"riderID"`
}

type PaymentEventSessionCreatedData struct {
	TripID    string  `json:"tripID"`
	SessionID string  `json:"sessionID"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
}

type PaymentTripResponseData struct {
	TripID   string  `json:"tripID"`
	UserID   string  `json:"userID"`
	DriverID string  `json:"driverID"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PaymentStatusUpdateData struct {
	TripID   string `json:"tripID"`
	UserID   string `json:"userID"`
	DriverID string `json:"driverID"`
}

type TripCancelledData struct {
	TripID string `json:"tripID"`
	UserID string `json:"userID"`
}
