package middleware

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

type tokenKeyType struct{}

var tokenKey = tokenKeyType{}

func extractBearerHeader(auth string) string {
	if auth == "" {
		return ""
	}

	scheme, token, ok := strings.Cut(auth, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}

	return token
}

func extractTokenFromFormBody(w http.ResponseWriter, r *http.Request) string {
	// Read at most 32K of the body to extract access token
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		resp.WriteHttpError(w, http.StatusRequestEntityTooLarge, "Request body too large")
		return ""
	}

	// Replace the body so handlers can read it again
	r.Body = io.NopCloser(bytes.NewReader(body))

	values, err := url.ParseQuery(string(body))

	// Make a debug log if there was a parse error for clarity
	// It is possible the payload is partially read, so an error makes sense
	// We'll try to get an auth token anyway; the debug message nudges to prefer Auth header instead
	if err != nil {
		if config.Debug() {
			log.Printf("debug: form body parse error during token extraction (consider using Auth header): %v", err)
		}
	}

	// micropub declares the parameter "auth_token" when providing the token in this manner
	return values.Get("auth_token")
}

// function ValidateTokenMiddleware wraps a downstream handler. At execution time,
// it extracts a Bearer token from the Authorization header, if any. If the Authorization
// header is not present, or does not contain a Bearer token, it aborts the request.
// If the token is present, it performs the VerifyAccessToken routine which makes a downstream
// http request to the defined token endpoint to validate the token.
func ValidateTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType, ok := util.ExtractMediaType(w, r)
		if !ok {
			return
		}

		token := extractBearerHeader(r.Header.Get("Authorization"))
		if token == "" && r.Method == http.MethodPost && contentType == "application/x-www-form-urlencoded" {
			// If token is not in header, method is post, and content type is x-www-form-urlencoded...
			// We need to check the body, unfortunately
			token = extractTokenFromFormBody(w, r)
		}

		token = strings.TrimSpace(token)
		if token == "" {
			resp.WriteHttpError(w, http.StatusUnauthorized, "An access token is required")
			return
		}

		details := auth.VerifyAccessToken(token)
		if details == nil {
			resp.WriteHttpError(w, http.StatusForbidden, "Token validation failed. Please try again with a valid token.")
			return
		}

		ctx := context.WithValue(r.Context(), tokenKey, details)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
