package get

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
)

type Service struct {
	Name  string `json:"name"`
	Url   string `json:"url"`
	Photo string `json:"photo"`
}

type SyndicateTo struct {
	Uid     string  `json:"uid"`
	Name    string  `json:"name"`
	Service Service `json:"service"`
}

type Config struct {
	MediaEndpoint string        `json:"media-endpoint"`
	SyndicateTo   []SyndicateTo `json:"syndicate-to"`
}

func HandleConfig(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	cfgOut := Config{
		MediaEndpoint: fmt.Sprintf("%v/media", cfg.Server.PublicUrl),
		SyndicateTo:   []SyndicateTo{},
	}

	err := json.NewEncoder(w).Encode(cfgOut)
	if err != nil {
		resp.WriteInternalServerError(w, "failed to encode configuration data")
		return
	}

	resp.WriteNoContent(w)
}
