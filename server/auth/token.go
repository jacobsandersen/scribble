package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

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

func VerifyAccessToken(cfg *config.Config, token string) *TokenDetails {
	if token == "" {
		log.Panicf("error: received empty token")
	}

	tokenEndpointUrl := cfg.Micropub.TokenEndpoint
	req, err := http.NewRequest(http.MethodGet, tokenEndpointUrl, nil)
	if err != nil {
		log.Fatal(fmt.Errorf("error: could not create http request for token endpoint: %w", err))
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(fmt.Errorf("error: failed to make http request to token endpoint: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if cfg.Debug {
			log.Printf("debug: token failed validation at token endpoint (%q)", token)
		}

		return nil
	}

	details := &TokenDetails{}
	err = json.NewDecoder(resp.Body).Decode(details)
	if err != nil {
		log.Println(fmt.Errorf("warning: token endpoint provided bad data, can not verify token: %w", err))
		return nil
	}

	if details.Me == "" {
		log.Println("warning: token endpoint did not include \"me\" information - cannot verify token")
		return nil
	}

	if !details.HasMe(cfg.Micropub.MeUrl) {
		if cfg.Debug {
			log.Printf("debug: received a valid token that did not belong to this instance! (%q)\n", token)
		}

		return nil
	}

	return details
}
