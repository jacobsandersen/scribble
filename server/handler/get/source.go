package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/resp"
)

func HandleSource(w http.ResponseWriter, r *http.Request) {
	resp.WriteNoContent(w)
}
