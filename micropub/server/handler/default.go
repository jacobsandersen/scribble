package handler

import (
	"net/http"

	"github.com/indieinfra/scribble/micropub/resp"
	"github.com/indieinfra/scribble/micropub/server/handler/get"
	"github.com/indieinfra/scribble/micropub/server/handler/post"
)

func HandleMicropub(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		get.DispatchGet(w, r)
	case http.MethodPost:
		post.DispatchPost(w, r)
	default:
		resp.WriteHttpError(w, http.StatusMethodNotAllowed, "micropub only supports GET or POST requests")
	}
}
