package content

import (
	"context"
	"testing"

	"github.com/indieinfra/scribble/server/util"
)

func TestNoopContentStoreLifecycle(t *testing.T) {
	store := &NoopContentStore{}
	ctx := context.Background()

	doc := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"name": []any{"Title"}}}

	url, created, err := store.Create(ctx, doc)
	if err != nil || url == "" || !created {
		t.Fatalf("unexpected create result: url=%q created=%v err=%v", url, created, err)
	}

	if _, err := store.Update(ctx, url, map[string][]any{"name": []any{"New"}}, map[string][]any{}, nil); err != nil {
		t.Fatalf("update returned error: %v", err)
	}

	if _, err := store.Update(ctx, url, map[string][]any{}, map[string][]any{}, []any{"del"}); err != nil {
		t.Fatalf("update with slice deletions returned error: %v", err)
	}

	if _, err := store.Update(ctx, url, map[string][]any{}, map[string][]any{}, map[string][]any{"remove": []any{"x"}}); err != nil {
		t.Fatalf("update with map deletions returned error: %v", err)
	}

	if err := store.Delete(ctx, url); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}

	if newURL, undeleted, err := store.Undelete(ctx, url); err != nil || newURL != url || undeleted {
		t.Fatalf("unexpected undelete result: %q %v %v", newURL, undeleted, err)
	}

	if got, err := store.Get(ctx, url); err != nil || got == nil || got.Url != url {
		t.Fatalf("unexpected get result: %+v err=%v", got, err)
	}

	exists, err := store.ExistsBySlug(ctx, "slug")
	if err != nil || exists {
		t.Fatalf("expected exists to return false without error")
	}
}
