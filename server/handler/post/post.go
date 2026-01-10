package post

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
)

func DispatchPost(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := ReadBody(cfg, w, r)
		if body == nil {
			return
		}

		actionRaw, ok := body["action"]
		if !ok {
			actionRaw = "create"
		}

		action, ok := actionRaw.(string)
		if !ok {
			resp.WriteInvalidRequest(w, fmt.Sprintf("Action must be a string, got %v", action))
			return
		}

		delete(body, "action")

		switch strings.ToLower(action) {
		case "create":
			Create(w, r, body)
		case "update":
			Update(w, r, body)
		case "delete":
			Delete(w, r, body, false)
		case "undelete":
			Delete(w, r, body, true)
		default:
			resp.WriteInvalidRequest(w, fmt.Sprintf("Unknown action: %q", action))
		}
	}
}
