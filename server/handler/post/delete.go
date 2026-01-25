package post

import (
	"net/http"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
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

	if !util.UrlIsSupported(st.Cfg.Content.PublicUrl, url) {
		resp.WriteInvalidRequest(w, "Invalid URL (not a supported destination)")
		return
	}

	if isUndelete {
		if !requireScope(w, r, auth.ScopeUndelete) {
			return
		}

		url, isNewUrl, err := st.ContentStore.Undelete(r.Context(), url)
		if err != nil {
			common.LogAndWriteError(w, r, "undelete content", err)
		} else if isNewUrl {
			resp.WriteCreated(w, url)
		} else {
			resp.WriteNoContent(w)
		}
	} else {
		if !requireScope(w, r, auth.ScopeDelete) {
			return
		}

		if err := st.ContentStore.Delete(r.Context(), url); err != nil {
			common.LogAndWriteError(w, r, "delete content", err)
		} else {
			resp.WriteNoContent(w)
		}
	}
}
