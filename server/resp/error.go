package resp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func WriteHttpError(w http.ResponseWriter, status int, message string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(ErrorResponse{
		Message: message,
	})

	if err != nil {
		http.Error(w, fmt.Errorf("Error: %q; additionally, an error was encountered while writing error response: %w", message, err).Error(), http.StatusInternalServerError)
	}
}
