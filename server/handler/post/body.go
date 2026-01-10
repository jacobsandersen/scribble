package post

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

func ReadBody(cfg *config.Config, w http.ResponseWriter, r *http.Request) map[string]any {
	_, contentType, ok := util.RequireValidMicropubContentType(w, r)
	if !ok {
		return nil
	}

	switch contentType {
	case "application/json":
		return readJsonBody(cfg, w, r)
	case "application/x-www-form-urlencoded":
		return readFormUrlEncodedBody(cfg, w, r)
	}

	return nil
}

func readJsonBody(cfg *config.Config, w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Server.Limits.MaxPayloadSize))
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		resp.WriteInvalidRequest(w, "Invalid JSON body")
		return nil
	}

	return out
}

func readFormUrlEncodedBody(cfg *config.Config, w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Server.Limits.MaxPayloadSize))
	if err := r.ParseForm(); err != nil {
		resp.WriteInvalidRequest(w, fmt.Sprintf("Invalid form body: %v", err))
		return nil
	}

	for key, values := range r.Form {
		switch len(values) {
		case 0:
			continue
		case 1:
			out[key] = values[0]
		default:
			arr := make([]any, len(values))
			for i, v := range values {
				arr[i] = v
			}
			out[key] = arr
		}
	}

	return out
}
