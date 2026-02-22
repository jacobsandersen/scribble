package micropubupload

import (
	"net/http"
	"time"

	"github.com/jacobsandersen/scribble/server/auth"
	"github.com/jacobsandersen/scribble/server/middleware"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
	"github.com/jacobsandersen/scribble/server/util"
)

func HandleMediaUpload(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, contentType, ok := util.RequireValidMediaContentType(w, r)
		if !ok {
			resp.WriteInvalidRequest(w, "Invalid Content-Type for media upload: "+contentType)
			return
		}

		limits := st.Cfg.Server.Limits
		maxMemory := int64(limits.MaxMultipartMem)
		maxSize := int64(limits.MaxFileSize)
		parsed, err := util.ParseMultipart(w, r, maxMemory, maxSize)
		if err != nil {
			resp.LogAndWriteError(w, r, "parse multipart", err)
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
			resp.LogAndWriteError(w, r, "generate path from pattern", err)
			return
		}

		err = st.MediaStore.Upload(r.Context(), file, fileKey)
		if err != nil {
			resp.LogAndWriteError(w, r, "upload media", err)
			return
		}

		resp.WriteCreated(w, fileUrl)
	}
}
