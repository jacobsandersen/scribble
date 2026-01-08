package post

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/server/resp"
)

func DispatchPost(w http.ResponseWriter, r *http.Request) {
	body := ReadBody(w, r)
	if body == nil {
		return
	}

	action, ok, err := body.GetString("action")
	if err != nil {
		resp.WriteHttpError(w, http.StatusBadRequest, fmt.Sprintf("unexpected error interpreting action: %v", err))
		return
	} else if !ok {
		action = "create"
	}

	switch strings.ToLower(action) {
	case "create":
		Create(w, r)
	case "update":
		Update(w, r)
	case "delete":
		Delete(w, r)
	case "undelete":
		Undelete(w, r)
	default:
		resp.WriteHttpError(w, http.StatusBadRequest, fmt.Sprintf("unknown action %q", action))
	}
}
