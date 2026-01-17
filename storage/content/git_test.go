package content

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	git "github.com/go-git/go-git/v6"
	gogitcfg "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"

	appconfig "github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

func newTestGitStore(t *testing.T) *GitContentStore {
	t.Helper()

	repoPath := setupRemoteRepo(t)

	cfg := &appconfig.GitContentStrategy{
		Repository: repoPath,
		Path:       "content",
		PublicUrl:  "https://example.test",
		Auth: appconfig.GitContentStrategyAuth{
			Method: "plain",
			Plain: &appconfig.UsernamePasswordAuth{
				Username: "user",
				Password: "pass",
			},
		},
	}

	store, err := NewGitContentStore(cfg)
	if err != nil {
		t.Fatalf("failed to create git content store: %v", err)
	}

	t.Cleanup(func() {
		_ = store.Cleanup()
	})

	return store
}

func setupRemoteRepo(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	workDir := filepath.Join(base, "work")
	bareDir := filepath.Join(base, "remote.git")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		t.Fatalf("failed to create bare dir: %v", err)
	}

	bareRepo, err := git.PlainInit(bareDir, true)
	if err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	workRepo, err := git.PlainInit(workDir, false)
	if err != nil {
		t.Fatalf("failed to init work repo: %v", err)
	}

	wt, err := workRepo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("failed to add seed file: %v", err)
	}

	commitHash, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("failed to commit seed: %v", err)
	}

	// Create main branch pointing at the seed commit
	mainRef := plumbing.NewBranchReferenceName("main")
	if err := workRepo.Storer.SetReference(plumbing.NewHashReference(mainRef, commitHash)); err != nil {
		t.Fatalf("failed to create main reference: %v", err)
	}
	if err := workRepo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, mainRef)); err != nil {
		t.Fatalf("failed to move HEAD to main: %v", err)
	}

	if _, err := workRepo.CreateRemote(&gogitcfg.RemoteConfig{Name: "origin", URLs: []string{bareDir}}); err != nil {
		t.Fatalf("failed to create remote: %v", err)
	}

	if err := workRepo.Push(&git.PushOptions{RemoteName: "origin", RefSpecs: []gogitcfg.RefSpec{"refs/heads/main:refs/heads/main"}}); err != nil {
		t.Fatalf("failed to push seed commit: %v", err)
	}

	if err := bareRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.NewBranchReferenceName("main"))); err != nil {
		t.Fatalf("failed to set bare head: %v", err)
	}

	return bareDir
}

func TestGitContentStore_CreateAndGet(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"post-1"},
			"name": {"Hello"},
		},
	}

	url, created, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created {
		t.Fatalf("expected created=false, got true")
	}

	got, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if !reflect.DeepEqual(doc, *got) {
		t.Fatalf("document mismatch: got %+v", got)
	}
}

func TestGitContentStore_Update(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"post-2"},
			"name": {"First"},
		},
	}

	url, _, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	replacements := map[string][]any{"name": {"Updated"}}
	additions := map[string][]any{"category": {"tech"}}

	if _, err := store.Update(ctx, url, replacements, additions, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if got.Properties["name"][0] != "Updated" {
		t.Fatalf("name not updated: %+v", got.Properties["name"])
	}

	if len(got.Properties["category"]) != 1 || got.Properties["category"][0] != "tech" {
		t.Fatalf("category not added: %+v", got.Properties["category"])
	}
}

func TestGitContentStore_DeleteUndelete(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"post-3"},
			"name": {"Hello"},
		},
	}

	url, _, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := store.Delete(ctx, url); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	got, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed after delete: %v", err)
	}

	if del := got.Properties["deleted"]; len(del) != 1 || del[0] != true {
		t.Fatalf("deleted flag not set: %+v", del)
	}

	if _, _, err := store.Undelete(ctx, url); err != nil {
		t.Fatalf("undelete failed: %v", err)
	}

	got, err = store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed after undelete: %v", err)
	}

	if del := got.Properties["deleted"]; len(del) != 1 || del[0] != false {
		t.Fatalf("deleted flag not cleared: %+v", del)
	}
}

func TestGitContentStore_NotFound(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "https://example.test/does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGitContentStore_ExistsBySlug(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug":    {"post-4"},
			"mp-slug": {"post-4"},
			"name":    {"Hello"},
		},
	}

	if _, _, err := store.Create(ctx, doc); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	exists, err := store.ExistsBySlug(ctx, "post-4")
	if err != nil {
		t.Fatalf("exists lookup failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected slug to exist")
	}

	missing, err := store.ExistsBySlug(ctx, "missing")
	if err != nil {
		t.Fatalf("exists lookup failed: %v", err)
	}
	if missing {
		t.Fatalf("expected missing slug to be false")
	}
}
