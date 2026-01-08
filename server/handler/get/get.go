package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/resp"
)

func DispatchGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	switch q {
	case "config":
		HandleConfig(w, r)
	case "source":
		HandleSource(w, r)
	case "syndicate-to":
		HandleSyndicateTo(w, r)
	default:
		resp.WriteHttpError(w, http.StatusBadRequest, "Unknown GET request")
	}
}
