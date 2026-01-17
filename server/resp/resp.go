package resp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"description"`
}

func WriteOK(w http.ResponseWriter, object any) {
	writeResp(w, http.StatusOK, object)
}

func WriteNoContent(w http.ResponseWriter) {
	writeResp(w, http.StatusNoContent, nil)
}

func WriteCreated(w http.ResponseWriter, location string) {
	if location != "" {
		w.Header().Add("Location", location)
	}

	writeResp(w, http.StatusCreated, nil)
}

func WriteAccepted(w http.ResponseWriter, location string) {
	if location != "" {
		w.Header().Add("Location", location)
	}

	writeResp(w, http.StatusAccepted, nil)
}

func WriteForbidden(w http.ResponseWriter, description string) {
	writeError(w, http.StatusForbidden, "forbidden", description)
}

func WriteInsufficientScope(w http.ResponseWriter, description string) {
	writeError(w, http.StatusUnauthorized, "insufficient_scope", description)
}

func WriteUnauthorized(w http.ResponseWriter, description string) {
	writeError(w, http.StatusUnauthorized, "unauthorized", description)
}

func WriteInvalidRequest(w http.ResponseWriter, description string) {
	writeError(w, http.StatusBadRequest, "invalid_request", description)
}

func WriteInternalServerError(w http.ResponseWriter, description string) {
	writeError(w, http.StatusInternalServerError, "internal_server_error", description)
}

func WriteNotFound(w http.ResponseWriter, description string) {
	writeError(w, http.StatusNotFound, "not_found", description)
}

func writeError(w http.ResponseWriter, status int, err string, description string) {
	writeResp(w, status, ErrorResponse{
		Error:       err,
		Description: description,
	})
}

func writeResp(w http.ResponseWriter, status int, object any) {
	haveObject := object != nil

	if haveObject {
		w.Header().Add("Content-Type", "application/json")
	}

	w.WriteHeader(status)

	if haveObject {
		err := json.NewEncoder(w).Encode(object)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to write standard HTTP response: %v", err), http.StatusInternalServerError)
		}
	}
}
