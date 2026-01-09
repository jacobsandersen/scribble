package post

import (
	"net/http"
	"strings"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

func Create(w http.ResponseWriter, r *http.Request, data map[string]any) {
	if !auth.RequestHasScope(r, auth.ScopeCreate) {
		resp.WriteInsufficientScope(w, "no create scope")
		return
	}

	ct, _ := util.ExtractMediaType(w, r)

	var document util.Mf2Document
	switch ct {
	case "application/json":
		document = normalizeJson(data)
	case "application/x-www-form-urlencoded":
		document = normalizeFormBody(data)
	}

	err := util.ValidateMf2(document)
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	url, err := content.ActiveContentStore.Create(r.Context(), document)
	if err != nil {
		resp.WriteInternalServerError(w, err.Error())
		return
	}

	resp.WriteCreated(w, url)
}

func normalizeJson(input map[string]any) util.Mf2Document {
	doc := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: content.MicropubProperties{},
	}

	if rawType, ok := input["type"]; ok {
		switch v := rawType.(type) {
		case string:
			doc.Type = []string{v}
		case []any:
			var types []string
			for _, t := range v {
				if s, ok := t.(string); ok {
					types = append(types, s)
				}
			}

			if len(types) > 0 {
				doc.Type = types
			}
		}
	}

	rawProps, ok := input["properties"]
	if !ok {
		return doc
	}

	props, ok := rawProps.(map[string]any)
	if !ok {
		return doc
	}

	for key, val := range props {
		switch v := val.(type) {
		case string:
			doc.Properties[key] = []any{v}
		case []any:
			doc.Properties[key] = normalizeJsonArray(v)
		case map[string]any:
			doc.Properties[key] = []any{normalizeJson(v)}
		}
	}

	return doc
}

func normalizeJsonArray(arr []any) []any {
	out := make([]any, 0, len(arr))

	for _, v := range arr {
		switch x := v.(type) {
		case string:
			out = append(out, x)
		case map[string]any:
			out = append(out, normalizeJson(x))
		}
	}

	return out
}

func normalizeFormBody(props map[string]any) util.Mf2Document {
	doc := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: content.MicropubProperties{},
	}

	for key, val := range props {
		if key == "h" {
			if s, ok := firstString(val); ok {
				doc.Type = []string{"h-" + s}
			}
			continue
		}

		if strings.HasSuffix(key, "[]") {
			key, _ = strings.CutSuffix(key, "[]")
		}

		values := coerceSlice(val)
		if len(values) == 0 {
			continue
		}

		doc.Properties[key] = values
	}

	return doc
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

func coerceSlice(v any) []any {
	var out []any

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
