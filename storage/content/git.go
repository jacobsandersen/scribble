package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

type GitContentStore struct {
	cfg       *config.GitContentStrategy
	auth      *transport.AuthMethod
	repo      *git.Repository
	tmpDir    string
	mu        sync.Mutex
	publicURL string
}

var NoErrFound error = errors.New("found")

func freshClone(cfg *config.GitContentStrategy, auth transport.AuthMethod) (string, *git.Repository, error) {
	tmpDir, err := os.MkdirTemp("", "scribble-*")
	if err != nil {
		return "", nil, err
	}

	repo, err := git.PlainClone(tmpDir, &git.CloneOptions{
		URL:  cfg.Repository,
		Auth: auth,
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}

	return tmpDir, repo, nil
}

func NewGitContentStore(cfg *config.GitContentStrategy) (*GitContentStore, error) {
	auth, err := buildGitAuth(cfg)
	if err != nil {
		return nil, err
	}

	tmpDir, repo, err := freshClone(cfg, auth)
	if err != nil {
		return nil, err
	}

	return &GitContentStore{
		cfg:       cfg,
		auth:      &auth,
		repo:      repo,
		tmpDir:    tmpDir,
		publicURL: normalizeBaseURL(cfg.PublicUrl),
	}, nil
}

func buildGitAuth(cfg *config.GitContentStrategy) (transport.AuthMethod, error) {
	switch cfg.Auth.Method {
	case "plain":
		return &http.BasicAuth{
			Username: cfg.Auth.Plain.Username,
			Password: cfg.Auth.Plain.Password,
		}, nil
	case "ssh":
		pubkeys, err := ssh.NewPublicKeysFromFile(cfg.Auth.Ssh.Username, cfg.Auth.Ssh.PrivateKeyFilePath, cfg.Auth.Ssh.Passphrase)

		if err != nil {
			return nil, fmt.Errorf("failed to prepare content git ssh authentication: %w", err)
		}

		return pubkeys, nil
	default:
		return nil, fmt.Errorf("invalid git authentication method %v", cfg.Auth.Method)
	}
}

func (cs *GitContentStore) reinit() error {
	// Remove old tmpDir
	os.RemoveAll(cs.tmpDir)

	tmpDir, repo, err := freshClone(cs.cfg, *cs.auth)
	if err != nil {
		return err
	}

	cs.tmpDir = tmpDir
	cs.repo = repo

	return nil
}

// Cleanup removes the cloned repository directory to free up disk space.
// Should be called when the application is shutting down.
func (cs *GitContentStore) Cleanup() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.tmpDir == "" {
		return nil
	}

	if err := os.RemoveAll(cs.tmpDir); err != nil {
		return fmt.Errorf("failed to cleanup git content store: %w", err)
	}

	cs.tmpDir = ""
	return nil
}

func (cs *GitContentStore) fetchAndFastForward(ctx context.Context) error {
	var lastErr error

	for range 3 {
		if err := cs.repo.FetchContext(ctx, &git.FetchOptions{Auth: *cs.auth}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			lastErr = err
			cs.reinit()
			continue
		}

		remoteRef, err := cs.repo.Reference(plumbing.NewRemoteReferenceName("origin", "main"), true)
		if err != nil {
			lastErr = err
			cs.reinit()
			continue
		}

		localRef, err := cs.repo.Reference(plumbing.NewBranchReferenceName("main"), true)
		if err != nil {
			lastErr = err
			cs.reinit()
			continue
		}

		if localRef.Hash() == remoteRef.Hash() {
			// Nothing to do
			return nil
		}

		if err := cs.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), remoteRef.Hash())); err != nil {
			lastErr = err
			cs.reinit()
			continue
		}

		wt, err := cs.repo.Worktree()
		if err != nil {
			lastErr = err
			cs.reinit()
			continue
		}

		if err := wt.Reset(&git.ResetOptions{
			Mode:   git.HardReset,
			Commit: remoteRef.Hash(),
		}); err != nil {
			lastErr = err
			cs.reinit()
			continue
		}

		return nil
	}

	return fmt.Errorf("could not fetch + fastforward after 3 retries: %w", lastErr)
}

func (cs *GitContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	// Get slug from "slug" property (set by post handler)
	slugProp, ok := doc.Properties["slug"]
	if !ok || len(slugProp) == 0 {
		return "", false, fmt.Errorf("document must have a slug property")
	}

	slug, ok := slugProp[0].(string)
	if !ok || slug == "" {
		return "", false, fmt.Errorf("slug property must be a non-empty string")
	}

	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", false, err
	}

	filename := slug + ".json"
	relPath := filepath.Join(cs.cfg.Path, filename)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err = cs.fetchAndFastForward(ctx); err != nil {
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

	_, err = wt.Commit(fmt.Sprintf("scribble(add): create content entry: %v", slug), &git.CommitOptions{
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

	return cs.publicURL + slug, false, nil
}

func (cs *GitContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	oldSlug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.fetchAndFastForward(ctx); err != nil {
		return url, fmt.Errorf("failed to update repo from remote: %w", err)
	}

	doc, err := cs.readDocumentBySlug(oldSlug)
	if err != nil {
		return url, err
	}
	if doc == nil {
		return url, ErrNotFound
	}

	applyMutations(doc, replacements, additions, deletions)

	// Check if slug needs to be recomputed
	var newSlug string
	if shouldRecomputeSlug(replacements, additions) {
		newSlug, err = computeNewSlug(doc, replacements)
		if err != nil {
			return url, err
		}

		// Update the slug property in the document
		doc.Properties["slug"] = []any{newSlug}
	} else {
		newSlug = oldSlug
	}

	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return url, err
	}

	oldFilename := oldSlug + ".json"
	newFilename := newSlug + ".json"
	oldRelPath := filepath.Join(cs.cfg.Path, oldFilename)
	newRelPath := filepath.Join(cs.cfg.Path, newFilename)
	oldFullPath := filepath.Join(cs.tmpDir, oldRelPath)
	newFullPath := filepath.Join(cs.tmpDir, newRelPath)

	wt, err := cs.repo.Worktree()
	if err != nil {
		return url, fmt.Errorf("failed to get worktree: %w", err)
	}

	// If slug changed, rename the file
	if newSlug != oldSlug {
		// Write to new file
		if err = os.WriteFile(newFullPath, jsonBytes, 0644); err != nil {
			return url, fmt.Errorf("failed to write new file: %w", err)
		}

		// Remove old file
		if err = os.Remove(oldFullPath); err != nil {
			return url, fmt.Errorf("failed to remove old file: %w", err)
		}

		// Stage both operations
		if _, err = wt.Add(newRelPath); err != nil {
			return url, fmt.Errorf("failed to add new file to git: %w", err)
		}

		if _, err = wt.Remove(oldRelPath); err != nil {
			return url, fmt.Errorf("failed to remove old file from git: %w", err)
		}

		_, err = wt.Commit(fmt.Sprintf("scribble(update): rename %v to %v", oldSlug, newSlug), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "scribble",
				Email: "scribble@local",
				When:  time.Now(),
			},
		})
	} else {
		// No slug change, just update in place
		if err = os.WriteFile(oldFullPath, jsonBytes, 0644); err != nil {
			return url, fmt.Errorf("failed to write file: %w", err)
		}

		if _, err = wt.Add(oldRelPath); err != nil {
			return url, fmt.Errorf("failed to add file to git: %w", err)
		}

		_, err = wt.Commit(fmt.Sprintf("scribble(update): update content entry: %v", oldSlug), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "scribble",
				Email: "scribble@local",
				When:  time.Now(),
			},
		})
	}

	if err != nil {
		return url, fmt.Errorf("failed to create commit: %w", err)
	}

	if err := cs.repo.PushContext(ctx, &git.PushOptions{Auth: *cs.auth}); err != nil {
		return url, fmt.Errorf("failed to push local: %w", err)
	}

	return cs.publicURL + newSlug, nil
}

func (cs *GitContentStore) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *GitContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := cs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (cs *GitContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return nil, err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.fetchAndFastForward(ctx); err != nil {
		return nil, fmt.Errorf("failed to update repo from remote: %w", err)
	}

	doc, err := cs.readDocumentBySlug(slug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNotFound
	}

	return doc, nil
}

func (cs *GitContentStore) readDocumentBySlug(slug string) (*util.Mf2Document, error) {
	head, err := cs.repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := cs.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	filename := slug + ".json"
	filePath := strings.TrimSuffix(cs.cfg.Path, "/") + "/" + filename

	file, err := tree.File(filePath)
	if err != nil {
		return nil, nil
	}

	r, err := file.Reader()
	if err != nil {
		return nil, nil
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil
	}

	var doc util.Mf2Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil
	}

	return &doc, nil
}

func (cs *GitContentStore) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.fetchAndFastForward(ctx); err != nil {
		return url, fmt.Errorf("failed to update repo from remote: %w", err)
	}

	doc, err := cs.readDocumentBySlug(slug)
	if err != nil {
		return url, err
	}
	if doc == nil {
		return url, ErrNotFound
	}

	applyMutations(doc, map[string][]any{"deleted": []any{deleted}}, nil, nil)

	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return url, err
	}

	filename := slug + ".json"
	relPath := filepath.Join(cs.cfg.Path, filename)
	fullPath := filepath.Join(cs.tmpDir, relPath)

	if err = os.WriteFile(fullPath, jsonBytes, 0644); err != nil {
		return url, fmt.Errorf("failed to write file: %w", err)
	}

	wt, err := cs.repo.Worktree()
	if err != nil {
		return url, fmt.Errorf("failed to get worktree: %w", err)
	}

	if _, err = wt.Add(relPath); err != nil {
		return url, fmt.Errorf("failed to add file to git: %w", err)
	}

	action := "delete"
	if !deleted {
		action = "undelete"
	}

	_, err = wt.Commit(fmt.Sprintf("scribble(%s): mark content entry as deleted=%v: %v", action, deleted, slug), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "scribble",
			Email: "scribble@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return url, fmt.Errorf("failed to create commit: %w", err)
	}

	if err := cs.repo.PushContext(ctx, &git.PushOptions{Auth: *cs.auth}); err != nil {
		return url, fmt.Errorf("failed to push local: %w", err)
	}

	return cs.publicURL + slug, nil
}

func (cs *GitContentStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.fetchAndFastForward(ctx); err != nil {
		return false, err
	}

	head, err := cs.repo.Head()
	if err != nil {
		return false, err
	}

	commit, err := cs.repo.CommitObject(head.Hash())
	if err != nil {
		return false, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return false, err
	}

	// Fast path: check for filename match to avoid deserializing documents.
	target := filepath.Join(strings.TrimSuffix(cs.cfg.Path, "/"), slug+".json")
	if _, err := tree.File(target); err == nil {
		return true, nil
	} else if !errors.Is(err, object.ErrFileNotFound) {
		return false, err
	}

	basePath := strings.TrimSuffix(cs.cfg.Path, "/") + "/"
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasPrefix(f.Name, basePath) {
			return nil
		}

		if !strings.HasSuffix(f.Name, ".json") {
			return nil
		}

		r, err := f.Reader()
		if err != nil {
			return err
		}

		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return err
		}

		var doc util.Mf2Document
		if err := json.Unmarshal(data, &doc); err != nil {
			// invalid JSON does not mean we should stop looking
			return nil
		}

		slugProps := doc.Properties["slug"]
		if len(slugProps) == 0 {
			return nil
		}

		docSlug, ok := slugProps[0].(string)
		if !ok {
			return nil
		}

		if strings.EqualFold(slug, docSlug) {
			return NoErrFound
		}

		return nil
	})

	if errors.Is(err, NoErrFound) {
		return true, nil
	}

	// err may be nil, meaning simply not found
	return false, err
}
