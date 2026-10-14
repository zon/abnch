// Package httpx writes the service API's JSON responses.
package httpx

import (
	"encoding/json"
	"net/http"
)

// errorBody is the JSON body of a service error response.
type errorBody struct {
	Error errorMessage `json:"error"`
}

// errorMessage carries the message a service error response reports.
type errorMessage struct {
	Message string `json:"message"`
}

// WriteJSON writes a service response with the given status and JSON body.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError writes a service error response with the given status and message.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, errorBody{Error: errorMessage{Message: message}})
}
