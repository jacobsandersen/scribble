package get

import (
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
)

func HandleSource(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	url := q.Get("url")
	if url == "" {
		resp.WriteInvalidRequest(w, "source requires a url")
		return
	}

	resp.WriteNoContent(w)
}
