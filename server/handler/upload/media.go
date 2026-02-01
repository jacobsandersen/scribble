package upload

import (
	"net/http"
	"time"

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
		parsed, err := util.ParseMultipart(w, r, maxMemory, maxSize)
		if err != nil {
			common.LogAndWriteError(w, r, "parse multipart", err)
			return
		}

		token := auth.PopAccessToken(parsed.Values)
		if token != "" && auth.GetToken(r.Context()) != nil {
			parsed.CloseFiles()
			resp.WriteInvalidRequest(w, "access token must appear in header or body, not both")
			return
		}

		r, ok = middleware.EnsureTokenForRequest(st.Cfg, w, r, token)
		if !ok {
			parsed.CloseFiles()
			return
		}

		defer parsed.CloseFiles()

		file := parsed.FileByKey("file")
		if file == nil {
			resp.WriteInvalidRequest(w, "no file uploaded with field name 'file'")
			return
		}

		fileUrl := st.MediaPathPattern.GenerateMedia(time.Now().Local(), file.FileExtension)
		fileKey, err := util.GetUrlPath(fileUrl)
		if err != nil {
			common.LogAndWriteError(w, r, "generate path from pattern", err)
			return
		}

		err = st.MediaStore.Upload(r.Context(), file, fileKey)
		if err != nil {
			common.LogAndWriteError(w, r, "upload media", err)
			return
		}

		resp.WriteCreated(w, fileUrl)
	}
}
