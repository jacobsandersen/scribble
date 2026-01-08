package util

import (
	"fmt"
	"mime"
	"net/http"
	"slices"

	"github.com/indieinfra/scribble/server/resp"
)

func RequireValidMicropubContentType(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	return requireValidContentType(w, r, []string{"application/json", "application/x-www-form-urlencoded"})
}

func RequireValidMediaContentType(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	return requireValidContentType(w, r, []string{"multipart/form-data"})
}

func ExtractMediaType(w http.ResponseWriter, r *http.Request) (string, bool) {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			"Content-Type must be specified",
		)

		return "", false
	}

	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			fmt.Errorf("Invalid Content-Type: %w", err).Error(),
		)

		return "", false
	}

	return mediaType, true
}

func requireValidContentType(w http.ResponseWriter, r *http.Request, valid []string) (string, string, bool) {
	if r.Method == http.MethodGet && r.Method != http.MethodHead {
		return r.Method, "", true
	}

	mediaType, ok := ExtractMediaType(w, r)
	if !ok {
		return r.Method, "", false
	}

	if slices.Contains(valid, mediaType) {
		return r.Method, mediaType, true
	} else {
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			fmt.Sprintf("Invalid-Content-Type: only %v allowed", valid),
		)
		return r.Method, mediaType, false
	}
}
