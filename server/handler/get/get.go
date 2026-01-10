package get

import (
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
)

func DispatchGet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		switch q {
		case "config":
			HandleConfig(cfg, w, r)
		case "source":
			HandleSource(cfg, w, r)
		case "syndicate-to":
			HandleSyndicateTo(cfg, w, r)
		default:
			resp.WriteInvalidRequest(w, fmt.Sprintf("Unknown query: %q", q))
		}
	}
}
