package auth

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/micropub/resp"
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
		method, contentType, ok := util.RequireValidContentType(w, r)
		if !ok {
			return
		}

		token := extractBearerHeader(r.Header.Get("Authorization"))
		if token == "" && method == http.MethodPost && contentType == "application/x-www-form-urlencoded" {
			// If token is not in header, method is post, and content type is x-www-form-urlencoded...
			// We need to check the body, unfortunately
			token = extractTokenFromFormBody(w, r)
		}

		token = strings.TrimSpace(token)
		if token == "" {
			resp.WriteHttpError(w, http.StatusUnauthorized, "An access token is required")
			return
		}

		details := VerifyAccessToken(token)
		if details == nil {
			resp.WriteHttpError(w, http.StatusForbidden, "Token validation failed. Please try again with a valid token.")
			return
		}

		ctx := context.WithValue(r.Context(), tokenKey, details)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
