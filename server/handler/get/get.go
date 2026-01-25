package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func DispatchGet(st *state.ScribbleState) http.HandlerFunc {
	handlers := map[string]func(*state.ScribbleState, http.ResponseWriter, *http.Request, body.QueryParams){
		"config":       HandleConfig,
		"source":       HandleSource,
		"category":     HandleCategory,
		"syndicate-to": HandleSyndicateTo,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		params := body.ReadQueryParams(r)
		query := params.GetFirst("q")
		if query != "" {
			if handler, ok := handlers[query]; ok {
				handler(st, w, r, params)
				return
			}
		}

		resp.WriteInvalidRequest(w, "Unknown GET request")
	}
}
