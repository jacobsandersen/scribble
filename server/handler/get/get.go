package get

import (
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func DispatchGet(st *state.ScribbleState) http.HandlerFunc {
	handlers := map[string]func(*state.ScribbleState, http.ResponseWriter, *http.Request){
		"config":       HandleConfig,
		"source":       HandleSource,
		"syndicate-to": HandleSyndicateTo,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if handler, ok := handlers[q]; ok {
			handler(st, w, r)
			return
		}

		resp.WriteInvalidRequest(w, fmt.Sprintf("Unknown query: %q", q))
	}
}
