package content

import (
	"context"
	"encoding/json"
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
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"

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

func TestBuildGitAuthPlain(t *testing.T) {
	cfg := &appconfig.GitContentStrategy{
		Auth: appconfig.GitContentStrategyAuth{
			Method: "plain",
			Plain:  &appconfig.UsernamePasswordAuth{Username: "u", Password: "p"},
		},
	}

	auth, err := buildGitAuth(cfg)
	if err != nil {
		t.Fatalf("expected plain auth to succeed: %v", err)
	}

	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected BasicAuth, got %T", auth)
	}
	if basic.Username != "u" || basic.Password != "p" {
		t.Fatalf("unexpected credentials: %+v", basic)
	}
}

func TestBuildGitAuthInvalidMethod(t *testing.T) {
	cfg := &appconfig.GitContentStrategy{Auth: appconfig.GitContentStrategyAuth{Method: "unknown"}}
	if _, err := buildGitAuth(cfg); err == nil {
		t.Fatalf("expected error for invalid method")
	}
}

func TestBuildGitAuthSSHError(t *testing.T) {
	cfg := &appconfig.GitContentStrategy{
		Auth: appconfig.GitContentStrategyAuth{
			Method: "ssh",
			Ssh:    &appconfig.SshKeyAuth{Username: "git", PrivateKeyFilePath: "/does/not/exist", Passphrase: ""},
		},
	}

	if _, err := buildGitAuth(cfg); err == nil {
		t.Fatalf("expected ssh auth to fail for missing key file")
	}
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
			"slug":     {"post-2"},
			"category": {"First"},
		},
	}

	url, _, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	replacements := map[string][]any{"category": {"Updated"}}
	additions := map[string][]any{"syndication": {"https://example.com"}}

	if _, err := store.Update(ctx, url, replacements, additions, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if got.Properties["category"][0] != "Updated" {
		t.Fatalf("category not updated: %+v", got.Properties["category"])
	}

	if len(got.Properties["syndication"]) != 1 || got.Properties["syndication"][0] != "https://example.com" {
		t.Fatalf("syndication not added: %+v", got.Properties["syndication"])
	}
}

func TestGitContentStore_UpdateSlugChange(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	// Test 1: Update name should trigger slug change
	doc1 := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"old-slug"},
			"name": {"Old Title"},
		},
	}

	url1, _, err := store.Create(ctx, doc1)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	newURL1, err := store.Update(ctx, url1, map[string][]any{"name": {"New Awesome Title"}}, nil, nil)
	if err != nil {
		t.Fatalf("update with name change failed: %v", err)
	}

	if newURL1 != "https://example.test/new-awesome-title" {
		t.Fatalf("expected new URL https://example.test/new-awesome-title, got %s", newURL1)
	}

	// Verify the new file exists and old file is gone
	oldPath := filepath.Join(store.tmpDir, store.cfg.Path, "old-slug.json")
	newPath := filepath.Join(store.tmpDir, store.cfg.Path, "new-awesome-title.json")

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old file should not exist: %s", oldPath)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new file should exist: %s", newPath)
	}

	// Verify we can retrieve by new URL
	got1, err := store.Get(ctx, newURL1)
	if err != nil {
		t.Fatalf("get by new URL failed: %v", err)
	}

	if got1.Properties["name"][0] != "New Awesome Title" {
		t.Fatalf("expected name 'New Awesome Title', got %v", got1.Properties["name"][0])
	}

	if got1.Properties["slug"][0] != "new-awesome-title" {
		t.Fatalf("expected slug 'new-awesome-title', got %v", got1.Properties["slug"][0])
	}

	// Test 2: Direct slug replacement
	doc2 := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"another-slug"},
			"name": {"Some Title"},
		},
	}

	url2, _, err := store.Create(ctx, doc2)
	if err != nil {
		t.Fatalf("create2 failed: %v", err)
	}

	newURL2, err := store.Update(ctx, url2, map[string][]any{"slug": {"custom-slug"}}, nil, nil)
	if err != nil {
		t.Fatalf("update with direct slug failed: %v", err)
	}

	if newURL2 != "https://example.test/custom-slug" {
		t.Fatalf("expected new URL https://example.test/custom-slug, got %s", newURL2)
	}

	// Verify the file rename happened
	oldPath2 := filepath.Join(store.tmpDir, store.cfg.Path, "another-slug.json")
	newPath2 := filepath.Join(store.tmpDir, store.cfg.Path, "custom-slug.json")

	if _, err := os.Stat(oldPath2); !os.IsNotExist(err) {
		t.Fatalf("old file should not exist: %s", oldPath2)
	}

	if _, err := os.Stat(newPath2); err != nil {
		t.Fatalf("new file should exist: %s", newPath2)
	}

	got2, err := store.Get(ctx, newURL2)
	if err != nil {
		t.Fatalf("get by new URL2 failed: %v", err)
	}

	if got2.Properties["slug"][0] != "custom-slug" {
		t.Fatalf("expected slug 'custom-slug', got %v", got2.Properties["slug"][0])
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
			"slug": {"post-4"},
			"name": {"Hello"},
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

func TestGitContentStore_ExistsBySlug_FallbackMatch(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	wt, err := store.repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	contentPath := filepath.Join(store.cfg.Path, "other.json")
	fullPath := filepath.Join(store.tmpDir, contentPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug": {"Post-Fallback"},
		},
	}

	data, _ := json.Marshal(doc)
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := wt.Add(contentPath); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("add fallback", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := store.repo.PushContext(ctx, &git.PushOptions{Auth: *store.auth}); err != nil {
		t.Fatalf("push: %v", err)
	}

	found, err := store.ExistsBySlug(ctx, "post-fallback")
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if !found {
		t.Fatalf("expected fallback slug match")
	}
}

func TestGitContentStore_ExistsBySlug_FetchError(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	if err := os.RemoveAll(store.cfg.Repository); err != nil {
		t.Fatalf("remove remote: %v", err)
	}

	if _, err := store.ExistsBySlug(ctx, "any"); err == nil {
		t.Fatalf("expected error when fetch fails")
	}
}

func TestGitContentStore_ExistsBySlug_IgnoresInvalidJSON(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	wt, err := store.repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	invalidPath := filepath.Join(store.cfg.Path, "invalid.json")
	fullPath := filepath.Join(store.tmpDir, invalidPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write invalid: %v", err)
	}

	if _, err := wt.Add(invalidPath); err != nil {
		t.Fatalf("add invalid: %v", err)
	}
	if _, err := wt.Commit("add invalid", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit invalid: %v", err)
	}

	if err := store.repo.PushContext(ctx, &git.PushOptions{Auth: *store.auth}); err != nil {
		t.Fatalf("push invalid: %v", err)
	}

	exists, err := store.ExistsBySlug(ctx, "nope")
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if exists {
		t.Fatalf("expected slug to be missing even with invalid json present")
	}
}

func TestGitContentStore_Reinit(t *testing.T) {
	store := newTestGitStore(t)
	oldDir := store.tmpDir

	if err := store.reinit(); err != nil {
		t.Fatalf("reinit failed: %v", err)
	}

	if store.tmpDir == "" || store.tmpDir == oldDir {
		t.Fatalf("expected tmpDir to be replaced")
	}

	if _, err := os.Stat(oldDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old temp dir not removed: %v", err)
	}
}

func TestGitContentStore_FetchAndFastForward_FastForward(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	// Prepare remote with a new commit after the initial clone.
	workDir := t.TempDir()
	clone, err := git.PlainClone(workDir, &git.CloneOptions{URL: store.cfg.Repository})
	if err != nil {
		t.Fatalf("clone remote: %v", err)
	}

	wt, err := clone.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	updated := []byte("updated\n")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), updated, 0644); err != nil {
		t.Fatalf("rewrite readme: %v", err)
	}

	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add readme: %v", err)
	}

	if _, err := wt.Commit("update readme", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit update: %v", err)
	}

	if err := clone.Push(&git.PushOptions{}); err != nil {
		t.Fatalf("push update: %v", err)
	}

	if err := store.fetchAndFastForward(ctx); err != nil {
		t.Fatalf("fetch and fast-forward: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(store.tmpDir, "README.md"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}

	if !reflect.DeepEqual(data, updated) {
		t.Fatalf("expected fast-forwarded file to match remote update")
	}
}

func TestGitContentStore_FetchAndFastForward_ReinitOnLocalRefFailure(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()
	oldDir := store.tmpDir

	if err := store.repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main")); err != nil {
		t.Fatalf("remove local ref: %v", err)
	}

	if err := store.fetchAndFastForward(ctx); err != nil {
		t.Fatalf("fetch and fast-forward: %v", err)
	}

	if store.tmpDir == oldDir {
		t.Fatalf("expected reinit to replace working directory")
	}

	if _, err := store.repo.Reference(plumbing.NewBranchReferenceName("main"), true); err != nil {
		t.Fatalf("expected main reference after reinit: %v", err)
	}
}

func TestGitContentStore_RollbackLastCommit(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	// Create initial document
	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug":    {"test-slug"},
			"content": {"initial content"},
		},
	}

	_, _, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Get current HEAD
	ref, err := store.repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	originalHead := ref.Hash()

	// Make a local change and commit (without pushing)
	store.mu.Lock()
	wt, err := store.repo.Worktree()
	if err != nil {
		store.mu.Unlock()
		t.Fatalf("failed to get worktree: %v", err)
	}

	testPath := filepath.Join(store.tmpDir, store.cfg.Path, "test-file.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0644); err != nil {
		store.mu.Unlock()
		t.Fatalf("failed to write test file: %v", err)
	}

	if _, err := wt.Add(filepath.Join(store.cfg.Path, "test-file.txt")); err != nil {
		store.mu.Unlock()
		t.Fatalf("failed to add test file: %v", err)
	}

	_, err = wt.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test",
			When:  time.Now(),
		},
	})
	if err != nil {
		store.mu.Unlock()
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify HEAD moved forward
	ref, err = store.repo.Head()
	if err != nil {
		store.mu.Unlock()
		t.Fatalf("failed to get new HEAD: %v", err)
	}
	newHead := ref.Hash()

	if originalHead == newHead {
		store.mu.Unlock()
		t.Fatalf("HEAD should have moved forward after commit")
	}

	// Now rollback
	if err := store.rollbackLastCommit(); err != nil {
		store.mu.Unlock()
		t.Fatalf("rollback failed: %v", err)
	}
	store.mu.Unlock()

	// Verify HEAD is back to original
	ref, err = store.repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD after rollback: %v", err)
	}
	rolledBackHead := ref.Hash()

	if originalHead != rolledBackHead {
		t.Fatalf("expected HEAD to be rolled back to %v, got %v", originalHead, rolledBackHead)
	}

	// Verify test file is gone from working directory
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatalf("test file should not exist after rollback")
	}
}

func TestGitContentStore_ResetToHead(t *testing.T) {
	store := newTestGitStore(t)
	ctx := context.Background()

	// Create initial document
	doc := util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"slug":    {"test-slug"},
			"content": {"initial content"},
		},
	}

	_, _, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	wt, err := store.repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Make some changes and stage them
	testPath := filepath.Join(store.tmpDir, store.cfg.Path, "dirty-file.txt")
	if err := os.WriteFile(testPath, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if _, err := wt.Add(filepath.Join(store.cfg.Path, "dirty-file.txt")); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Modify existing file
	existingPath := filepath.Join(store.tmpDir, store.cfg.Path, "test-slug.json")
	if err := os.WriteFile(existingPath, []byte(`{"modified": true}`), 0644); err != nil {
		t.Fatalf("failed to modify existing file: %v", err)
	}

	// Verify files exist and are dirty
	if _, err := os.Stat(testPath); err != nil {
		t.Fatalf("dirty file should exist before reset: %v", err)
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !status.IsClean() {
		// Good - we have dirty changes
	} else {
		t.Fatalf("expected dirty working tree before reset")
	}

	// Now reset to HEAD - should clean everything up
	if err := store.resetToHead(wt); err != nil {
		t.Fatalf("resetToHead failed: %v", err)
	}

	// Verify working tree is clean
	status, err = wt.Status()
	if err != nil {
		t.Fatalf("failed to get status after reset: %v", err)
	}

	if !status.IsClean() {
		t.Fatalf("expected clean working tree after reset, got: %v", status)
	}

	// Verify dirty file is gone
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatalf("dirty file should not exist after reset")
	}

	// Verify existing file is restored to original content
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing file after reset: %v", err)
	}

	var restored util.Mf2Document
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal restored file: %v", err)
	}

	if restored.Properties["content"][0] != "initial content" {
		t.Fatalf("expected original content to be restored, got: %v", restored.Properties["content"][0])
	}
}
