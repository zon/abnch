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

// WriteError writes a service error response with the given status and message.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorMessage{Message: message}})
}
