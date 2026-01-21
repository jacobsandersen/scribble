package upload

import (
	"net/http"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/middleware"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

func HandleMediaUpload(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := util.RequireValidMediaContentType(w, r)
		if !ok {
			return
		}

		maxMemory := int64(st.Cfg.Server.Limits.MaxMultipartMem)
		maxSize := int64(st.Cfg.Server.Limits.MaxFileSize)
		values, file, header, _, ok := util.ParseMultipartWithFirstFile(w, r, maxMemory, maxSize, []string{"file"}, true)
		if !ok {
			return
		}

		token := auth.PopAccessToken(values)
		if token != "" && auth.GetToken(r.Context()) != nil {
			if file != nil {
				file.Close()
			}
			resp.WriteInvalidRequest(w, "access token must appear in header or body, not both")
			return
		}
		r, ok = middleware.EnsureTokenForRequest(st.Cfg, w, r, token)
		if !ok {
			if file != nil {
				file.Close()
			}
			return
		}
		defer file.Close()

		url, err := st.MediaStore.Upload(r.Context(), &file, header)
		if err != nil {
			common.LogAndWriteError(w, r, "upload media", err)
			return
		}

		resp.WriteCreated(w, url)
	}
}
