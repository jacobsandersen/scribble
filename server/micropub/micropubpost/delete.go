package micropubpost

import (
	"net/http"

	"github.com/jacobsandersen/scribble/server/auth"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
	"github.com/jacobsandersen/scribble/server/util"
)

func Delete(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, data map[string]any, isUndelete bool) {
	urlRaw, ok := data["url"]
	if !ok {
		resp.WriteInvalidRequest(w, "URL to (un)delete must be specified")
		return
	}

	url, ok := urlRaw.(string)
	if !ok {
		resp.WriteInvalidRequest(w, "URL to delete must be a string")
		return
	}

	if !util.UrlIsInstance(st.Cfg.Content.ContentUrl, url) {
		resp.WriteInvalidRequest(w, "Invalid URL (not a supported destination)")
		return
	}

	if isUndelete {
		if !requireScope(w, r, auth.ScopeUndelete) {
			return
		}

		if err := st.ContentStore.Undelete(r.Context(), url); err != nil {
			resp.LogAndWriteError(w, r, "undelete content", err)
		} else {
			resp.WriteNoContent(w)
		}
	} else {
		if !requireScope(w, r, auth.ScopeDelete) {
			return
		}

		if err := st.ContentStore.Delete(r.Context(), url); err != nil {
			resp.LogAndWriteError(w, r, "delete content", err)
		} else {
			resp.WriteNoContent(w)
		}
	}
}
