package post

import (
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

func Create(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, pb *body.ParsedBody) {
	if !requireScope(w, r, auth.ScopeCreate) {
		return
	}

	ct, _ := util.ExtractMediaType(w, r)

	document, err := buildDocument(ct, pb.Data)
	if err != nil {
		resp.WriteInvalidRequest(w, err.Error())
		return
	}

	for _, pf := range pb.Files {
		if pf.Header == nil || pf.File == nil {
			continue
		}

		mediaProperty := pf.Field
		if mediaProperty == "" || mediaProperty == "file" {
			mediaProperty = mediaPropertyForUpload(pf.Header)
		}

		url, err := st.MediaStore.Upload(r.Context(), &pf.File, pf.Header)
		if err != nil {
			common.LogAndWriteError(w, r, "upload media", err)
			return
		}

		document.Properties[mediaProperty] = append(document.Properties[mediaProperty], url)
	}

	suggestedSlug := deriveSuggestedSlug(&document)

	finalSlug, err := ensureUniqueSlug(r.Context(), st.ContentStore, suggestedSlug)
	if err != nil {
		common.LogAndWriteError(w, r, "slug lookup", err)
		return
	}

	document.Properties["slug"] = []any{finalSlug}

	url, now, err := st.ContentStore.Create(r.Context(), document)
	if err != nil {
		common.LogAndWriteError(w, r, "create content", err)
		return
	}

	if now {
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
	case "application/x-www-form-urlencoded":
		doc = normalizeFormBody(data)
		delete(doc.Properties, "access_token")
	case "multipart/form-data":
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

func ensureUniqueSlug(ctx context.Context, store content.Store, slug string) (string, error) {
	exists, err := store.ExistsBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	if !exists {
		return slug, nil
	}

	suffix, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return slug + "-" + suffix.String(), nil
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

func mediaPropertyForUpload(header *multipart.FileHeader) string {
	if header == nil {
		return "photo"
	}

	contentType := strings.ToLower(header.Header.Get("Content-Type"))
	if contentType == "" {
		if ext := strings.ToLower(filepath.Ext(header.Filename)); ext != "" {
			contentType = mime.TypeByExtension(ext)
		}
	}

	switch {
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "photo"
	}
}
