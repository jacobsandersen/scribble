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
		resp.WriteInvalidRequest(w, fmt.Sprintf("Unexpected error while reading action: %v", err))
		return
	} else if !ok {
		action = "create"
	}

	switch strings.ToLower(action) {
	case "create":
		Create(w, r, body)
	case "update":
		Update(w, r, body)
	case "delete":
		Delete(w, r, body)
	case "undelete":
		Undelete(w, r, body)
	default:
		resp.WriteInvalidRequest(w, fmt.Sprintf("Unknown action: %q", action))
	}
}
