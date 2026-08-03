// Package httpx provides HTTP helpers for JSON APIs: request decoding,
// response writing, and a unified error envelope. All consumers (api, auth)
// use these helpers so that HTTP wiring lives in one place.
package httpx

import (
	"encoding/json"
	"net/http"
)

// ErrorBody is the unified JSON error response envelope.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a machine-readable code and a human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON sets the Content-Type header, writes the status code, and encodes
// v as JSON to the response writer.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a unified JSON error response with the given HTTP status,
// machine-readable code, and human-readable message.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{
		Error: ErrorDetail{Code: code, Message: message},
	})
}

// DecodeJSON reads and decodes the request body as JSON into v. It returns
// the decoding error; the caller is responsible for sending an error response.
func DecodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
