package post

import (
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage"
)

func Create(w http.ResponseWriter, r *http.Request, b *MicropubData) {
	ct, _ := util.ExtractMediaType(w, r)

	var props map[string]any
	if ct == "application/x-www-form-urlencoded" {
		props = normalizeFormBody(b.Properties)
	} else {
		props = b.Properties
	}

	err := util.ValidateMf2(props)
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	url, err := storage.ActiveContentStore.Create(r.Context(), nil)
	if err != nil {
		resp.WriteInternalServerError(w, err.Error())
	}

	resp.WriteCreated(w, url)
}

func normalizeFormBody(props map[string]any) map[string]any {
	normalized := map[string]any{
		"type":       []string{"h-entry"},
		"properties": map[string]any{},
	}

	outProps := normalized["properties"].(map[string]any)

	for key, val := range props {
		if key == "h" {
			if s, ok := firstString(val); ok {
				normalized["type"] = []string{"h-" + s}
			}
			continue
		}

		if strings.HasSuffix(key, "[]") {
			base, _ := strings.CutSuffix(key, "[]")
			outProps[base] = coerceStringSlice(val)
			continue
		}

		switch v := val.(type) {
		case string:
			outProps[key] = v
		case []any:
			if len(v) == 1 {
				if s, ok := v[0].(string); ok {
					outProps[key] = s
				} else {
					outProps[key] = coerceStringSlice(v)
				}
			} else {
				outProps[key] = coerceStringSlice(v)
			}
		}
	}

	return normalized
}

func firstString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []any:
		if len(x) > 0 {
			if s, ok := x[0].(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

func coerceStringSlice(v any) []string {
	var out []string

	switch x := v.(type) {
	case string:
		out = append(out, x)
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}

	return out
}
