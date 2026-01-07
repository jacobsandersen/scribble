package auth

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/micropub/resp"
	"github.com/indieinfra/scribble/micropub/scope"
	"github.com/indieinfra/scribble/micropub/server/util"
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
	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxBytesFormUrlEncoded()))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		resp.WriteHttpError(w, http.StatusRequestEntityTooLarge, "Request body too large")
		return ""
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	values, err := url.ParseQuery(string(body))
	if err != nil {
		resp.WriteHttpError(w, http.StatusBadRequest, "Failed to read form body to get access token")
		return ""
	}

	return values.Get("access_token")
}

// function ValidateTokenMiddleware wraps a downstream handler. At execution time,
// it extracts a Bearer token from the Authorization header, if any. If the Authorization
// header is not present, or does not contain a Bearer token, it aborts the request.
// If the token is present, it performs the VerifyAccessToken routine which makes a downstream
// http request to the defined token endpoint to validate the token.
func ValidateTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, contentType, ok := util.RequireValidContentType(w, r)
		if !ok {
			return
		}

		token := extractBearerHeader(r.Header.Get("Authorization"))
		if token == "" && method != http.MethodGet && contentType == "application/x-www-form-urlencoded" {
			token = extractTokenFromFormBody(w, r)
		}

		token = strings.TrimSpace(token)
		if token == "" {
			resp.WriteHttpError(w, http.StatusUnauthorized, "An access token is required")
			return
		}

		details := VerifyAccessToken(token)
		if details == nil {
			resp.WriteHttpError(w, http.StatusUnauthorized, "Token validation failed. Please try again with a valid token.")
			return
		}

		ctx := context.WithValue(r.Context(), tokenKey, details)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// function RequireScopeMiddleware wraps a downstream handler. At execution time,
// the middleware expects a valid token to be available in the request context.
// The middleware will access the stored token details and validate the token
// contains the required scopes. Without the required scopes, the middleware will
// abort the request.
func RequireScopeMiddleware(scopes []scope.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			details, ok := r.Context().Value(tokenKey).(*TokenDetails)
			if !ok {
				resp.WriteHttpError(w, http.StatusUnauthorized, "Request is missing token")
				return
			}

			for _, scope := range scopes {
				if !details.HasScope(scope) {
					if config.Debug() {
						log.Printf("debug: received a valid token, but failed scope check (want %v, have %q)", scope, details.Scope)
					}

					resp.WriteHttpError(w, http.StatusForbidden, "Insufficient scope for request.")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
