package post

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func DispatchPost(st *state.ScribbleState) http.HandlerFunc {
	handlers := map[string]func(*state.ScribbleState, http.ResponseWriter, *http.Request, map[string]any){
		"create": Create,
		"update": Update,
		"delete": func(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, body map[string]any) {
			Delete(st, w, r, body, false)
		},
		"undelete": func(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, body map[string]any) {
			Delete(st, w, r, body, true)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		body := ReadBody(st.Cfg, w, r)
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

		if handler, ok := handlers[strings.ToLower(action)]; ok {
			handler(st, w, r, body)
			return
		}

		resp.WriteInvalidRequest(w, fmt.Sprintf("Unknown action: %q", action))
	}
}

func requireScope(w http.ResponseWriter, r *http.Request, scope auth.Scope) bool {
	if !auth.RequestHasScope(r, scope) {
		resp.WriteInsufficientScope(w, fmt.Sprintf("no %s scope", scope.String()))
		return false
	}
	return true
}
