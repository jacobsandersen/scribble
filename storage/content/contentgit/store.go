package contentgit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/google/uuid"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

type GitContentStore struct {
	cfg    *config.GitContentStrategy
	auth   *transport.AuthMethod
	repo   *git.Repository
	tmpDir string
	mu     sync.Mutex
}

func NewGitContentStore(cfg *config.GitContentStrategy) (*GitContentStore, error) {
	auth, err := BuildGitAuth(cfg)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "scribble-*")
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainClone(tmpDir, &git.CloneOptions{
		URL:  cfg.Repository,
		Auth: auth,
	})
	if err != nil {
		return nil, err
	}

	return &GitContentStore{
		cfg:    cfg,
		auth:   &auth,
		repo:   repo,
		tmpDir: tmpDir,
	}, nil
}

func (cs *GitContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", false, err
	}

	contentId, err := uuid.NewRandom()
	if err != nil {
		return "", false, err
	}

	filename := contentId.String() + ".json"
	relPath := filepath.Join(cs.cfg.Path, filename)

	// Prevent races for filesystem access
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Make sure we are up to date with remote, in case other sources have pushed
	if err = cs.repo.FetchContext(ctx, &git.FetchOptions{Auth: *cs.auth}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", false, fmt.Errorf("failed to update repo from remote: %w", err)
	}

	fullPath := filepath.Join(cs.tmpDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", false, fmt.Errorf("failed to create required directory structure: %w", err)
	}

	if err = os.WriteFile(fullPath, jsonBytes, 0644); err != nil {
		return "", false, fmt.Errorf("failed to write file: %w", err)
	}

	wt, err := cs.repo.Worktree()
	if err != nil {
		return "", false, fmt.Errorf("failed to get new worktree: %w", err)
	}

	if _, err = wt.Add(relPath); err != nil {
		return "", false, fmt.Errorf("failed to add file to git: %w", err)
	}

	_, err = wt.Commit("scribble(add): create content entry", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "scribble",
			Email: "scribble@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to create commit: %w", err)
	}

	if err := cs.repo.PushContext(ctx, &git.PushOptions{Auth: *cs.auth}); err != nil {
		return "", false, fmt.Errorf("failed to push local: %w", err)
	}

	return cs.cfg.PublicUrl + "/" + filename, false, nil
}

func (cs *GitContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	return url, nil
}

func (cs *GitContentStore) Delete(ctx context.Context, url string) error {
	return nil
}

func (cs *GitContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	return url, false, nil
}

func (cs *GitContentStore) Get(ctx context.Context, url string) (*content.ContentObject, error) {
	return &content.ContentObject{
		Url:  url,
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"name":    {"This is a bogus title"},
			"content": {"This is bogus content, sentence one", "sentence two!"},
		},
		Deleted: false,
	}, nil
}
