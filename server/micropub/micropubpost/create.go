package micropubpost

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/micropub/micropubcommon"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

func Create(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, pb *micropubcommon.ParsedBody) {
	if !requireScope(w, r, auth.ScopeCreate) {
		return
	}

	ct, ok := util.ExtractMediaType(w, r)
	if !ok {
		resp.WriteInvalidRequest(w, "missing or invalid Content-Type")
		return
	}

	document, err := buildDocument(ct, pb.Data)
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	for _, pf := range pb.Files {
		if pf.Header == nil || pf.File == nil {
			continue
		}

		objectUrl := st.MediaPathPattern.GenerateMedia(time.Now(), pf.FileExtension)
		objectKey, err := util.GetUrlPath(objectUrl)
		if err != nil {
			resp.LogAndWriteError(w, r, "derive s3 object key", err)
			return
		}

		err = st.MediaStore.Upload(r.Context(), pf, strings.TrimPrefix(objectUrl, objectKey))
		if err != nil {
			resp.LogAndWriteError(w, r, "upload media", err)
			return
		}

		document.Properties[pf.Field] = append(document.Properties[pf.Field], objectUrl)

		pf.File.Close()
	}

	timeNow := util.CurrentLocalTime()
	timeStr := util.TimeToRFC3339(&timeNow)

	slug, err := content.EnsureUniqueSlug(r.Context(), st.ContentStore, deriveSuggestedSlug(&document))
	if err != nil {
		resp.LogAndWriteError(w, r, "slug creation", err)
		return
	}

	document.SetProp("slug", slug)

	visibility, ok := document.GetFirstStringProp("visibility")
	if ok && strings.EqualFold(visibility, "unlisted") {
		document.SetProp("slug", uuid.New().String()) // override slug for unlisted posts
	}

	if !document.HasProp("created_at") {
		document.SetProp("created_at", timeStr)
	}

	if !document.HasProp("updated_at") {
		document.SetProp("updated_at", timeStr)
	}

	immediate, err := st.ContentStore.Create(r.Context(), document)
	if err != nil {
		resp.LogAndWriteError(w, r, "create content", err)
		return
	}

	url, err := st.ContentPathPattern.GenerateContent(timeNow, slug)
	if err != nil {
		resp.LogAndWriteError(w, r, "generate content URL", err)
		return
	}

	if immediate {
		resp.WriteCreated(w, url)
	} else {
		resp.WriteAccepted(w, url)
	}
}

func buildDocument(contentType string, data map[string]any) (util.Mf2Document, error) {
	var doc util.Mf2Document

	switch contentType {
	case "application/json":
		doc = normalizeJson(data)
	case "multipart/form-data":
	case "application/x-www-form-urlencoded":
		doc = normalizeFormBody(data)
		delete(doc.Properties, "access_token")
	default:
		return util.Mf2Document{}, fmt.Errorf("unsupported content type %q", contentType)
	}

	if err := util.ValidateMf2(doc); err != nil {
		return util.Mf2Document{}, err
	}

	return doc, nil
}

func deriveSuggestedSlug(doc *util.Mf2Document) string {
	suggestedSlug := processMpProperties(doc)
	if suggestedSlug != "" {
		return suggestedSlug
	}

	if generated := util.GenerateSlug(*doc); generated != "" {
		return generated
	}

	return uuid.NewString()
}

func normalizeJson(input map[string]any) util.Mf2Document {
	doc := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: util.MicroformatProperties{},
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
			// Preserve maps as-is for embedded objects like {html: ["..."], value: ["..."]}
			doc.Properties[key] = []any{v}
		case nil:
			// Skip nil values
		default:
			// Preserve other types (numbers, booleans, etc.)
			doc.Properties[key] = []any{v}
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
			// Preserve maps as-is (e.g., {html: ["..."], value: ["..."]})
			// Don't recursively normalize them to avoid losing structure
			out = append(out, x)
		case nil:
			// Skip nil values
		default:
			// Preserve other types (numbers, booleans, etc.)
			out = append(out, x)
		}
	}

	return out
}

func normalizeFormBody(props map[string]any) util.Mf2Document {
	doc := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: util.MicroformatProperties{},
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

		if _, exists := doc.Properties[key]; !exists {
			doc.Properties[key] = values
		} else {
			doc.Properties[key] = append(doc.Properties[key], values...)
		}
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

// extractStringFromProperty extracts the first string value from an MF2 property ([]any)
func extractStringFromProperty(values []any) string {
	for _, val := range values {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// processMpProperties handles server command properties (mp-*) and removes them from the document.
// Returns the suggested slug from mp-slug if present, otherwise returns empty string.
func processMpProperties(doc *util.Mf2Document) string {
	var suggestedSlug string

	// Extract mp-slug if present
	if mpSlugProp, ok := doc.Properties["mp-slug"]; ok {
		suggestedSlug = extractStringFromProperty(mpSlugProp)
	}

	// Collect mp-* keys first to avoid modifying map during iteration
	var mpKeys []string
	for key := range doc.Properties {
		if strings.HasPrefix(key, "mp-") {
			mpKeys = append(mpKeys, key)
		}
	}

	// Remove all mp-* (server command) properties per spec
	for _, key := range mpKeys {
		delete(doc.Properties, key)
	}

	return suggestedSlug
}

func coerceSlice(v any) []any {
	var out []any

	switch x := v.(type) {
	case []any:
		for _, e := range x {
			// Preserve all non-nil types
			if e != nil {
				out = append(out, e)
			}
		}
	default:
		// Preserve single non-nil values
		if x != nil {
			out = append(out, x)
		}
	}

	return out
}
