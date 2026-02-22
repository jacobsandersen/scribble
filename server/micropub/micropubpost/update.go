package micropubpost

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jacobsandersen/scribble/server/auth"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
	"github.com/jacobsandersen/scribble/server/util"
	"github.com/jacobsandersen/scribble/storage/content"
)

func Update(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, data map[string]any) {
	if !requireScope(w, r, auth.ScopeUpdate) {
		return
	}

	ct, _ := util.ExtractMediaType(w, r)
	if ct != "application/json" {
		resp.WriteInvalidRequest(w, "Update may only be processed via JSON body")
		return
	}

	url, err := getStringField(data, "url")
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	if !util.UrlIsInstance(st.Cfg.Content.ContentUrl, url) {
		resp.WriteInvalidRequest(w, "Invalid URL (not a supported destination)")
		return
	}

	oldSlug, err := util.GetSlug(st.Cfg.Content.ContentUrl, url)
	if err != nil {
		resp.WriteInvalidRequest(w, fmt.Sprintf("Could not extract slug from URL: %v", err))
		return
	}

	replacements, err := getMapOfStringToSlice(data, "replace")
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	additions, err := getMapOfStringToSlice(data, "add")
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	deletions, err := getDeletions(data)
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	doc, err := st.ContentStore.Update(r.Context(), url, replacements, additions, deletions)
	if err != nil {
		resp.LogAndWriteError(w, r, "update content", err)
		return
	}

	slug, err := content.ExtractSlug(*doc)
	if err != nil {
		resp.LogAndWriteError(w, r, "extract slug after update", err)
		resp.WriteNoContent(w) // Pray and return; we don't know if the URL changed
		return
	}

	if !strings.EqualFold(slug, oldSlug) {
		newUrl, err := st.ContentPathPattern.ReplaceSlugParam(url, slug)
		if err != nil {
			resp.LogAndWriteError(w, r, "generate new URL after slug change", err)
			resp.WriteNoContent(w) // We know the url changed, but can't generate the new one
			return
		}

		resp.WriteCreated(w, newUrl)
		return
	}

	resp.WriteNoContent(w)
}

func getStringField(data map[string]any, key string) (string, error) {
	raw, ok := data[key]
	if !ok {
		return "", fmt.Errorf("missing required field %q", key)
	}

	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%q must be a string", key)
	}

	return s, nil
}

func getMapOfStringToSlice(data map[string]any, key string) (map[string][]any, error) {
	out := map[string][]any{}
	raw, ok := data[key]
	if !ok {
		return out, nil
	}

	tmp, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q must be an object mapping property to array of values", key)
	}

	for k, v := range tmp {
		switch arr := v.(type) {
		case []any:
			out[k] = arr
		case string:
			out[k] = []any{arr}
		default:
			return nil, fmt.Errorf("%q.%q must be an array or string", key, k)
		}
	}

	return out, nil
}

func getDeletions(data map[string]any) (any, error) {
	raw, ok := data["delete"]
	if !ok {
		return nil, nil
	}

	// Could be []any (of property names) or map[string][]any (values to remove)
	switch v := raw.(type) {
	case []any:
		props := make([]string, 0, len(v))
		for i, p := range v {
			s, ok := p.(string)
			if !ok {
				return nil, fmt.Errorf("delete[%d] must be a string", i)
			}
			props = append(props, s)
		}
		return props, nil
	case map[string]any:
		out := map[string][]any{}
		for k, val := range v {
			switch arr := val.(type) {
			case []any:
				out[k] = arr
			case string:
				out[k] = []any{arr}
			default:
				return nil, fmt.Errorf("delete.%q must be string or array", k)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("delete must be array or object")
	}
}
