package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/resp"
)

func HandleSyndicateTo(w http.ResponseWriter, r *http.Request) {
	resp.WriteNoContent(w)
}
