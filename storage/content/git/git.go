package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	"github.com/google/uuid"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
	storageutil "github.com/indieinfra/scribble/storage/util"
)

var NoErrFound = errors.New("found")

type StoreImpl struct {
	cfg       *config.GitContentStrategy
	auth      *transport.AuthMethod
	repo      *git.Repository
	tmpDir    string
	mu        sync.Mutex
	publicURL string
}

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

func NewGitContentStore(cfg *config.GitContentStrategy) (*StoreImpl, error) {
	auth, err := buildGitAuth(cfg)
	if err != nil {
		return nil, err
	}

	tmpDir, repo, err := freshClone(cfg, auth)
	if err != nil {
		return nil, err
	}

	return &StoreImpl{
		cfg:       cfg,
		auth:      &auth,
		repo:      repo,
		tmpDir:    tmpDir,
		publicURL: storageutil.NormalizeBaseURL(cfg.PublicUrl),
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

func (cs *StoreImpl) reinit() error {
	// Remove old tmpDir
	if err := os.RemoveAll(cs.tmpDir); err != nil {
		log.Printf("failed to remove tmp dir: %v", err)
	}

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
func (cs *StoreImpl) Cleanup() error {
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

func (cs *StoreImpl) fetchAndFastForward(ctx context.Context) error {
	var lastErr error

	for range 3 {
		if err := cs.repo.FetchContext(ctx, &git.FetchOptions{Auth: *cs.auth}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		remoteRef, err := cs.repo.Reference(plumbing.NewRemoteReferenceName("origin", "main"), true)
		if err != nil {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		localRef, err := cs.repo.Reference(plumbing.NewBranchReferenceName("main"), true)
		if err != nil {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		if localRef.Hash() == remoteRef.Hash() {
			// Nothing to do
			return nil
		}

		if err := cs.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), remoteRef.Hash())); err != nil {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		wt, err := cs.repo.Worktree()
		if err != nil {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		if err := wt.Reset(&git.ResetOptions{
			Mode:   git.HardReset,
			Commit: remoteRef.Hash(),
		}); err != nil {
			lastErr = err
			if err := cs.reinit(); err != nil {
				lastErr = fmt.Errorf("%w: could not renit: %w", lastErr, err)
				break
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("could not fetch + fast-forward: %w", lastErr)
}

func (cs *StoreImpl) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
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

func (cs *StoreImpl) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
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
		return url, content.ErrNotFound
	}

	content.ApplyMutations(doc, replacements, additions, deletions)

	// Check if slug needs to be recomputed
	var newSlug string
	if content.ShouldRecomputeSlug(replacements, additions) {
		proposedSlug, err := content.ComputeNewSlug(doc, replacements)
		if err != nil {
			return url, err
		}

		// CRITICAL: Check for collision BEFORE making any writes to disk
		// We already hold the lock, so use the unlocked version
		if proposedSlug != oldSlug {
			exists, err := cs.existsBySlugUnlocked(proposedSlug)
			if err != nil {
				return url, fmt.Errorf("failed to check slug collision: %w", err)
			}

			if exists {
				// Collision detected - append UUID to make it unique
				proposedSlug = fmt.Sprintf("%s-%s", proposedSlug, uuid.New().String())

				// Sanity check the UUID-suffixed slug
				exists, err = cs.existsBySlugUnlocked(proposedSlug)
				if err != nil {
					return url, fmt.Errorf("failed to check unique slug: %w", err)
				}
				if exists {
					return url, fmt.Errorf("slug collision persists even after UUID suffix: %s", proposedSlug)
				}
			}
		}

		newSlug = proposedSlug
		// Update the slug property in the document with the final unique slug
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

	// If slug changed, use single atomic commit with both operations.
	// This ensures clean git history and avoids intermediate inconsistent states.
	// If push fails, we roll back the commit to prevent data loss.
	if newSlug != oldSlug {
		// Write new file
		if err = os.WriteFile(newFullPath, jsonBytes, 0644); err != nil {
			return url, fmt.Errorf("failed to write new file: %w", err)
		}

		// Remove old file from filesystem
		if err = os.Remove(oldFullPath); err != nil {
			// Failed to remove old file - clean up new file we just wrote
			_ = os.Remove(newFullPath)
			return url, fmt.Errorf("failed to remove old file: %w", err)
		}

		// Stage new file
		if _, err = wt.Add(newRelPath); err != nil {
			// Failed to stage new file - restore working directory from HEAD
			if cleanErr := cs.resetToHead(wt); cleanErr != nil {
				return url, fmt.Errorf("failed to stage new file and failed to restore from HEAD: stage_error=%w, restore_error=%v", err, cleanErr)
			}
			return url, fmt.Errorf("failed to stage new file (restored from HEAD): %w", err)
		}

		// Stage removal of old file
		if _, err = wt.Remove(oldRelPath); err != nil {
			// Failed to stage removal - restore working directory from HEAD
			if cleanErr := cs.resetToHead(wt); cleanErr != nil {
				return url, fmt.Errorf("failed to stage removal and failed to restore from HEAD: stage_error=%w, restore_error=%v", err, cleanErr)
			}
			return url, fmt.Errorf("failed to stage removal (restored from HEAD): %w", err)
		}

		// Create single atomic commit with both operations
		_, err = wt.Commit(fmt.Sprintf("scribble(update): rename %v to %v", oldSlug, newSlug), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "scribble",
				Email: "scribble@local",
				When:  time.Now(),
			},
		})
		if err != nil {
			// Commit failed - clean up staged changes and restore working directory
			if resetErr := cs.resetToHead(wt); resetErr != nil {
				return url, fmt.Errorf("failed to create commit and failed to clean up staged changes: commit_error=%w, reset_error=%v", err, resetErr)
			}
			return url, fmt.Errorf("failed to create commit (changes reverted): %w", err)
		}

		// Push commit to remote
		if err := cs.repo.PushContext(ctx, &git.PushOptions{Auth: *cs.auth}); err != nil {
			// Push failed - roll back the commit to restore original state
			// This prevents data loss by ensuring we don't have unpushed local changes
			if resetErr := cs.rollbackLastCommit(); resetErr != nil {
				// Reset failed - reinitialize repo to get back to clean remote state
				if reinitErr := cs.reinit(); reinitErr != nil {
					return url, fmt.Errorf("failed to push, rollback failed, and reinit failed (data preserved on remote): push_error=%w, rollback_error=%v, reinit_error=%v", err, resetErr, reinitErr)
				}
				return url, fmt.Errorf("failed to push, rolled back via reinit (data preserved on remote): %w", err)
			}
			return url, fmt.Errorf("failed to push, rolled back successfully (no changes made): %w", err)
		}
	} else {
		// No slug change, just update in place
		if err = os.WriteFile(oldFullPath, jsonBytes, 0644); err != nil {
			return url, fmt.Errorf("failed to write file: %w", err)
		}

		if _, err = wt.Add(oldRelPath); err != nil {
			// Failed to stage - restore file from HEAD
			if cleanErr := cs.resetToHead(wt); cleanErr != nil {
				return url, fmt.Errorf("failed to stage file and failed to restore from HEAD: stage_error=%w, restore_error=%v", err, cleanErr)
			}
			return url, fmt.Errorf("failed to stage file (restored from HEAD): %w", err)
		}

		_, err = wt.Commit(fmt.Sprintf("scribble(update): update content entry: %v", oldSlug), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "scribble",
				Email: "scribble@local",
				When:  time.Now(),
			},
		})
		if err != nil {
			// Commit failed - clean up staged changes and restore working directory
			if resetErr := cs.resetToHead(wt); resetErr != nil {
				return url, fmt.Errorf("failed to create commit and failed to clean up staged changes: commit_error=%w, reset_error=%v", err, resetErr)
			}
			return url, fmt.Errorf("failed to create commit (changes reverted): %w", err)
		}

		// Push commit to remote
		if err := cs.repo.PushContext(ctx, &git.PushOptions{Auth: *cs.auth}); err != nil {
			// Push failed - roll back the commit to restore original state
			if resetErr := cs.rollbackLastCommit(); resetErr != nil {
				// Reset failed - reinitialize repo to get back to clean remote state
				if reinitErr := cs.reinit(); reinitErr != nil {
					return url, fmt.Errorf("failed to push, rollback failed, and reinit failed (data preserved on remote): push_error=%w, rollback_error=%v, reinit_error=%v", err, resetErr, reinitErr)
				}
				return url, fmt.Errorf("failed to push, rolled back via reinit (data preserved on remote): %w", err)
			}
			return url, fmt.Errorf("failed to push, rolled back successfully (no changes made): %w", err)
		}
	}

	return cs.publicURL + newSlug, nil
}

func (cs *StoreImpl) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *StoreImpl) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := cs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (cs *StoreImpl) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
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
		return nil, content.ErrNotFound
	}

	return doc, nil
}

func (cs *StoreImpl) readDocumentBySlug(slug string) (*util.Mf2Document, error) {
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

func (cs *StoreImpl) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
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
		return url, content.ErrNotFound
	}

	content.ApplyMutations(doc, map[string][]any{"deleted": []any{deleted}}, nil, nil)

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

func (cs *StoreImpl) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.fetchAndFastForward(ctx); err != nil {
		return false, err
	}

	return cs.existsBySlugUnlocked(slug)
}

// existsBySlugUnlocked checks if a slug exists WITHOUT acquiring the mutex.
// MUST be called with cs.mu already locked.
func (cs *StoreImpl) existsBySlugUnlocked(slug string) (bool, error) {
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

// rollbackLastCommit performs a hard reset to undo the last commit.
// This is used when a push fails to restore the repository to its pre-commit state.
// Returns an error if the reset fails.
func (cs *StoreImpl) rollbackLastCommit() error {
	wt, err := cs.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree for rollback: %w", err)
	}

	// Get the current HEAD
	ref, err := cs.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD for rollback: %w", err)
	}

	// Get the commit object
	commit, err := cs.repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit object for rollback: %w", err)
	}

	// Get parent commit (HEAD~1)
	if commit.NumParents() == 0 {
		return fmt.Errorf("cannot rollback: commit has no parent")
	}

	parentCommit, err := commit.Parent(0)
	if err != nil {
		return fmt.Errorf("failed to get parent commit for rollback: %w", err)
	}

	// Hard reset to parent commit
	err = wt.Reset(&git.ResetOptions{
		Commit: parentCommit.Hash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset to parent commit: %w", err)
	}

	return nil
}

// resetToHead performs a hard reset to HEAD, discarding all staged changes and
// restoring the working directory to match the current commit.
// If reset fails, attempts to reinit the repository from remote as a last resort.
// This is used to clean up after failed commit or staging operations.
func (cs *StoreImpl) resetToHead(wt *git.Worktree) error {
	ref, err := cs.repo.Head()
	if err != nil {
		// Can't get HEAD - reinit to get back to clean remote state
		if reinitErr := cs.reinit(); reinitErr != nil {
			return fmt.Errorf("failed to get HEAD and failed to reinit: head_error=%w, reinit_error=%v", err, reinitErr)
		}
		return fmt.Errorf("failed to get HEAD, reinitialized from remote: %w", err)
	}

	err = wt.Reset(&git.ResetOptions{
		Commit: ref.Hash(),
		Mode:   git.HardReset,
	})
	if err != nil {
		// Reset failed - reinit to get back to clean remote state
		if reinitErr := cs.reinit(); reinitErr != nil {
			return fmt.Errorf("failed to reset to HEAD and failed to reinit: reset_error=%w, reinit_error=%v", err, reinitErr)
		}
		return fmt.Errorf("failed to reset to HEAD, reinitialized from remote: %w", err)
	}

	return nil
}
