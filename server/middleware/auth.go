package middleware

import (
	"bytes"
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

func extractTokenFromFormBody(cfg *config.Config, w http.ResponseWriter, r *http.Request) string {
	// Read at most 32K of the body to extract access token
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		resp.WriteInvalidRequest(w, "Request too large")
		return ""
	}

	// Replace the body so handlers can read it again
	r.Body = io.NopCloser(bytes.NewReader(body))

	values, err := url.ParseQuery(string(body))

	// Make a debug log if there was a parse error for clarity
	// It is possible the payload is partially read, so an error makes sense
	// We'll try to get an auth token anyway; the debug message nudges to prefer Auth header instead
	if err != nil {
		if cfg.Debug {
			rl := util.WithRequest(log.Default(), r, "")
			rl.Infof("form body parse error during token extraction (consider using Auth header): %v", err)
		}
	}

	return values.Get("access_token")
}

// function ValidateTokenMiddleware wraps a downstream handler. At execution time,
// it extracts a Bearer token from the Authorization header, if any. If the Authorization
// header is not present, or does not contain a Bearer token, it aborts the request.
// If the token is present, it performs the VerifyAccessToken routine which makes a downstream
// http request to the defined token endpoint to validate the token.
func ValidateTokenMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if r.Method != http.MethodGet {
			contentType, ok := util.ExtractMediaType(w, r)
			if !ok {
				return
			}

			token = extractBearerHeader(r.Header.Get("Authorization"))
			if token == "" && r.Method == http.MethodPost && contentType == "application/x-www-form-urlencoded" {
				// If token is not in header, method is post, and content type is x-www-form-urlencoded...
				// We need to check the body, unfortunately
				token = extractTokenFromFormBody(cfg, w, r)
			}
		} else {
			token = extractBearerHeader(r.Header.Get("Authorization"))
		}

		token = strings.TrimSpace(token)
		if token == "" {
			resp.WriteUnauthorized(w, "An access token is required")
			return
		}

		details := auth.VerifyAccessToken(cfg, token)
		if details == nil {
			resp.WriteForbidden(w, "Token validation failed")
			return
		}

		rl := util.WithRequest(log.Default(), r, details.Me)
		ctx := util.ContextWithLogger(r.Context(), rl)
		next.ServeHTTP(w, r.WithContext(auth.AddToken(ctx, details)))
	})
}
