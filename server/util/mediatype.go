package util

import (
	"mime"
	"net/http"
	"slices"
)

func RequireValidMicropubContentType(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	return requireValidContentType(w, r, []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"})
}

func RequireValidMediaContentType(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	return requireValidContentType(w, r, []string{"multipart/form-data"})
}

func ExtractMediaType(w http.ResponseWriter, r *http.Request) (string, bool) {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return "", false
	}

	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
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
		return r.Method, mediaType, false
	}
}
