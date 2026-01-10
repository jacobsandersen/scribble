package get

import (
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
)

func HandleSyndicateTo(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	resp.WriteNoContent(w)
}
