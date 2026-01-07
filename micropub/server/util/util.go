package util

import (
	"fmt"
	"mime"
	"net/http"

	"github.com/indieinfra/scribble/micropub/resp"
)

func RequireValidContentType(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	if r.Method == http.MethodGet && r.Method != http.MethodHead {
		return r.Method, "", true
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			"Content-Type must be specified",
		)

		return r.Method, "", false
	}

	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			fmt.Errorf("Invalid Content-Type: %w", err).Error(),
		)

		return r.Method, "", false
	}

	switch mediaType {
	case "application/json", "application/x-www-form-urlencoded", "multipart/form-data":
		return r.Method, mediaType, true
	default:
		resp.WriteHttpError(
			w,
			http.StatusUnsupportedMediaType,
			"Invalid-Content-Type: only application/json, application/x-www-form-urlencoded, or multipart/form-data",
		)

		return r.Method, "", false
	}
}
