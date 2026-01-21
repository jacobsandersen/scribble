package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/indieinfra/scribble/config"
)

type tokenKeyType struct{}

var tokenKey = tokenKeyType{}

type TokenDetails struct {
	Me       string `json:"me"`
	ClientId string `json:"client_id"`
	Scope    string `json:"scope"`
	IssuedAt uint   `json:"issued_at"`
	Nonce    int    `json:"nonce"`
}

// ExtractBearerToken extracts a Bearer token from an Authorization header value.
// Returns an empty string if the header is not present, malformed, or not a Bearer token.
func ExtractBearerToken(auth string) string {
	if auth == "" {
		return ""
	}

	scheme, token, ok := strings.Cut(auth, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}

	return token
}

// PopAccessToken extracts the first string access_token value from a map and removes the key.
// Returns an empty string if not present or not a string.
func PopAccessToken(values map[string]any) string {
	if values == nil {
		return ""
	}

	v, ok := values["access_token"]
	if !ok {
		return ""
	}

	delete(values, "access_token")

	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				return s
			}
		}
	}

	return ""
}

func AddToken(ctx context.Context, details *TokenDetails) context.Context {
	return context.WithValue(ctx, tokenKey, details)
}

func GetToken(ctx context.Context) *TokenDetails {
	token, ok := ctx.Value(tokenKey).(*TokenDetails)
	if !ok {
		return nil
	}

	return token
}

func (details *TokenDetails) String() string {
	return fmt.Sprintf("TokenDetails{me=%v, clientId=%v, scope=%v, issuedAt=%v, nonce=%v}", details.Me, details.ClientId, details.Scope, details.IssuedAt, details.Nonce)
}

func (details *TokenDetails) HasScope(scope Scope) bool {
	return slices.Contains(strings.Split(strings.ToLower(details.Scope), " "), strings.ToLower(scope.String()))
}

func (details *TokenDetails) HasMe(me string) bool {
	me = strings.TrimSuffix(strings.TrimSpace(me), "/") + "/"
	meDetails := strings.TrimSuffix(strings.TrimSpace(details.Me), "/") + "/"
	return strings.EqualFold(me, meDetails)
}

var (
	ErrEmptyToken        = errors.New("received empty token")
	ErrTokenEndpointFail = errors.New("failed to contact token endpoint")
)

func VerifyAccessToken(cfg *config.Config, token string) (*TokenDetails, error) {
	if token == "" {
		return nil, ErrEmptyToken
	}

	tokenEndpointUrl := cfg.Micropub.TokenEndpoint
	req, err := http.NewRequest(http.MethodGet, tokenEndpointUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create http request for token endpoint: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	// Create HTTP client with 10 second timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTokenEndpointFail, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if cfg.Debug {
			log.Printf("debug: token failed validation at token endpoint (%q)", token)
		}

		return nil, nil
	}

	details := &TokenDetails{}
	err = json.NewDecoder(resp.Body).Decode(details)
	if err != nil {
		log.Println(fmt.Errorf("warning: token endpoint provided bad data, can not verify token: %w", err))
		return nil, nil
	}

	if details.Me == "" {
		log.Println("warning: token endpoint did not include \"me\" information - cannot verify token")
		return nil, nil
	}

	if !details.HasMe(cfg.Micropub.MeUrl) {
		if cfg.Debug {
			log.Printf("debug: received a valid token that did not belong to this instance! (%q)\n", token)
		}

		return nil, nil
	}

	return details, nil
}
