package post

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/middleware"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func DispatchPost(st *state.ScribbleState) http.HandlerFunc {
	handlers := map[string]func(*state.ScribbleState, http.ResponseWriter, *http.Request, *body.ParsedBody){
		"create": Create,
		"update": func(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, pb *body.ParsedBody) {
			Update(st, w, r, pb.Data)
		},
		"delete": func(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, pb *body.ParsedBody) {
			Delete(st, w, r, pb.Data, false)
		},
		"undelete": func(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, pb *body.ParsedBody) {
			Delete(st, w, r, pb.Data, true)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		parsed, ok := body.ReadBody(st.Cfg, w, r)
		if !ok {
			return
		}
		if parsed.AccessToken != "" && auth.GetToken(r.Context()) != nil {
			for _, pf := range parsed.Files {
				if pf.File != nil {
					err := pf.File.Close()
					if err != nil {
						log.Printf("Error closing file: %v", err)
					}
				}
			}
			resp.WriteBadRequest(w, "access token must appear in header or body, not both")
			return
		}
		r, ok = middleware.EnsureTokenForRequest(st.Cfg, w, r, parsed.AccessToken)
		if !ok {
			return
		}
		for _, pf := range parsed.Files {
			if pf.File != nil {
				defer pf.File.Close()
			}
		}

		actionRaw, ok := parsed.Data["action"]
		if !ok {
			actionRaw = "create"
		}

		action, ok := actionRaw.(string)
		if !ok {
			resp.WriteInvalidRequest(w, fmt.Sprintf("Action must be a string, got %v", action))
			return
		}

		delete(parsed.Data, "action")

		if handler, ok := handlers[strings.ToLower(action)]; ok {
			handler(st, w, r, parsed)
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
