package micropubget

import (
	"fmt"
	"net/http"

	"github.com/jacobsandersen/scribble/server/body"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
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

func HandleConfig(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	cfgOut := Config{
		MediaEndpoint: fmt.Sprintf("%v/micropub/media", st.Cfg.Server.PublicUrl),
		SyndicateTo:   []SyndicateTo{},
	}

	resp.WriteOK(w, cfgOut)
}
