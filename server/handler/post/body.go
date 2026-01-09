package post

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

func ReadBody(w http.ResponseWriter, r *http.Request) map[string]any {
	_, contentType, ok := util.RequireValidMicropubContentType(w, r)
	if !ok {
		return nil
	}

	switch contentType {
	case "application/json":
		return readJsonBody(w, r)
	case "application/x-www-form-urlencoded":
		return readFormUrlEncodedBody(w, r)
	}

	return nil
}

func readJsonBody(w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxPayloadSize()))
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		resp.WriteInvalidRequest(w, "Invalid JSON body")
		return nil
	}

	return out
}

func readFormUrlEncodedBody(w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxPayloadSize()))
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
