package post

import (
	"net/http"

	"github.com/indieinfra/scribble/micropub/resp"
)

func DispatchPost(w http.ResponseWriter, r *http.Request) {
	body := ReadBody(w, r)
	if body == nil {
		resp.WriteHttpError(w, http.StatusInternalServerError, "Failed to read body from request")
		return
	}

	// Continue
}
