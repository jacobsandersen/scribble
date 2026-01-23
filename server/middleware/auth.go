package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

// ValidateTokenMiddleware wraps a downstream handler. At execution time,
// it extracts a Bearer token from the Authorization header, if any. If the Authorization
// header is not present, or does not contain a Bearer token, it aborts the request.
// If the token is present, it performs the VerifyAccessToken routine which makes a downstream
// http request to the defined token endpoint to validate the token.
func ValidateTokenMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.ExtractBearerToken(r.Header.Get("Authorization"))

		token = strings.TrimSpace(token)
		if token == "" {
			if r.Method == http.MethodGet {
				resp.WriteUnauthorized(w, "An access token is required")
				return
			}
			// For non-GET requests, allow handlers to pull tokens from the body.
			next.ServeHTTP(w, r)
			return
		}

		details, err := auth.VerifyAccessToken(cfg, token)
		if err != nil {
			log.Printf("error verifying access token: %v", err)
			resp.WriteInternalServerError(w, "Failed to verify token")
			return
		}
		if details == nil {
			resp.WriteForbidden(w, "Token validation failed")
			return
		}

		rl := util.WithRequest(log.Default(), r, details.Me)
		ctx := util.ContextWithLogger(r.Context(), rl)
		next.ServeHTTP(w, r.WithContext(auth.AddToken(ctx, details)))
	})
}

// EnsureTokenForRequest attaches validated token details to the request context using the provided
// token string when middleware has not already set them. It prefers existing context tokens and
// returns an updated request pointer.
func EnsureTokenForRequest(cfg *config.Config, w http.ResponseWriter, r *http.Request, token string) (*http.Request, bool) {
	if auth.GetToken(r.Context()) != nil {
		return r, true
	}

	token = strings.TrimSpace(token)
	if token == "" {
		resp.WriteUnauthorized(w, "An access token is required")
		return nil, false
	}

	details, err := auth.VerifyAccessToken(cfg, token)
	if err != nil {
		log.Printf("error verifying access token: %v", err)
		resp.WriteInternalServerError(w, "Failed to verify token")
		return nil, false
	}
	if details == nil {
		resp.WriteForbidden(w, "Token validation failed")
		return nil, false
	}

	rl := util.WithRequest(log.Default(), r, details.Me)
	ctx := util.ContextWithLogger(r.Context(), rl)
	return r.WithContext(auth.AddToken(ctx, details)), true
}
