package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v6"
	gogitcfg "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
)

// withToken injects a permissive token into the request context to bypass remote token verification.
func withToken(cfg *config.Config, next http.Handler) http.Handler {
	details := &auth.TokenDetails{Me: cfg.Micropub.MeUrl, Scope: "create update delete undelete read media", ClientId: "test-client"}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(auth.AddToken(r.Context(), details)))
	})
}

func newIntegrationState(t *testing.T) (*state.ScribbleState, func()) {
	t.Helper()

	repoPath := setupRemoteRepo(t)

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "git",
			Git: &config.GitContentStrategy{
				Repository: repoPath,
				Path:       "content",
				PublicUrl:  "https://example.test",
				Auth: config.GitContentStrategyAuth{
					Method: "plain",
					Plain:  &config.UsernamePasswordAuth{Username: "user", Password: "pass"},
				},
			},
		},
		Media: config.Media{Strategy: "s3"},
	}

	store, err := content.NewGitContentStore(cfg.Content.Git)
	if err != nil {
		t.Fatalf("failed to create git content store: %v", err)
	}

	st := &state.ScribbleState{Cfg: cfg, ContentStore: store, MediaStore: &media.NoopMediaStore{}}

	cleanup := func() {
		_ = store.Cleanup()
	}

	return st, cleanup
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

	commitHash, err := wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}})
	if err != nil {
		t.Fatalf("failed to commit seed: %v", err)
	}

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

func TestMicropub_CreateUpdateDeleteFlow(t *testing.T) {
	st, cleanup := newIntegrationState(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.Handle("POST /", withToken(st.Cfg, post.DispatchPost(st)))
	mux.Handle("GET /", withToken(st.Cfg, get.DispatchGet(st)))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	// Create post via form-encoded body
	form := url.Values{}
	form.Set("name", "Hello World")
	form.Set("content", "Integration body")

	createResp, err := client.Post(srv.URL+"/", "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	createResp.Body.Close()

	if createResp.StatusCode != http.StatusAccepted && createResp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected create status: %d", createResp.StatusCode)
	}

	loc := createResp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("expected Location header from create")
	}

	// Fetch via source
	sourceURL := srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	srcResp, err := client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	if srcResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected source status: %d", srcResp.StatusCode)
	}

	var doc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode source response: %v", err)
	}

	if got := doc.Properties["name"]; len(got) == 0 || got[0] != "Hello World" {
		t.Fatalf("unexpected name property: %+v", got)
	}

	// Update name
	updateBody := map[string]any{
		"action": "update",
		"url":    loc,
		"replace": map[string]any{
			"name": []any{"Updated Name"},
		},
	}

	buf, _ := json.Marshal(updateBody)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")

	updResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	updResp.Body.Close()

	// Updating name triggers slug recomputation, so expect 201 Created with new Location
	if updResp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected update status: %d", updResp.StatusCode)
	}

	// Get the new location if slug changed
	newLoc := updResp.Header.Get("Location")
	if newLoc != "" {
		loc = newLoc
		sourceURL = srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	}

	// Verify update via source
	srcResp, err = client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	var docAfter content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&docAfter); err != nil {
		t.Fatalf("failed to decode updated doc: %v", err)
	}

	if got := docAfter.Properties["name"]; len(got) == 0 || got[0] != "Updated Name" {
		t.Fatalf("name not updated: %+v", got)
	}

	// Delete
	deleteBody := map[string]any{"action": "delete", "url": loc}
	buf, _ = json.Marshal(deleteBody)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")

	delResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	delResp.Body.Close()

	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d", delResp.StatusCode)
	}

	// Verify deleted flag
	srcResp, err = client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	var deletedDoc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&deletedDoc); err != nil {
		t.Fatalf("failed to decode deleted doc: %v", err)
	}

	if del := deletedDoc.Properties["deleted"]; len(del) == 0 || del[0] != true {
		t.Fatalf("deleted flag not set: %+v", del)
	}
}

func TestMicropub_MultipartCreateWithFile(t *testing.T) {
	st, cleanup := newIntegrationState(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.Handle("POST /", withToken(st.Cfg, post.DispatchPost(st)))
	mux.Handle("GET /", withToken(st.Cfg, get.DispatchGet(st)))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "Multipart Title")
	_ = writer.WriteField("content", "Multipart Body")
	fileWriter, err := writer.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	fileWriter.Write([]byte("hello"))
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/", &buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	createResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	createResp.Body.Close()

	if createResp.StatusCode != http.StatusAccepted && createResp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected create status: %d", createResp.StatusCode)
	}

	loc := createResp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("expected Location header from create")
	}

	sourceURL := srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	srcResp, err := client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	if srcResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected source status: %d", srcResp.StatusCode)
	}

	var doc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode source response: %v", err)
	}

	if got := doc.Properties["photo"]; len(got) == 0 || got[0] != "https://noop.example.org/noop" {
		t.Fatalf("expected photo to be uploaded url, got %+v", got)
	}
}

func TestMicropub_MultipartCreateWithPhotoField(t *testing.T) {
	st, cleanup := newIntegrationState(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.Handle("POST /", withToken(st.Cfg, post.DispatchPost(st)))
	mux.Handle("GET /", withToken(st.Cfg, get.DispatchGet(st)))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "Photo Field Title")
	_ = writer.WriteField("content", "Photo Field Body")
	photoWriter, err := writer.CreateFormFile("photo", "pic.jpg")
	if err != nil {
		t.Fatalf("failed to create photo form file: %v", err)
	}
	photoWriter.Write([]byte("jpg-bytes"))
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/", &buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	createResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	createResp.Body.Close()

	if createResp.StatusCode != http.StatusAccepted && createResp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected create status: %d", createResp.StatusCode)
	}

	loc := createResp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("expected Location header from create")
	}

	sourceURL := srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	srcResp, err := client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	if srcResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected source status: %d", srcResp.StatusCode)
	}

	var doc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode source response: %v", err)
	}

	if got := doc.Properties["photo"]; len(got) == 0 || got[0] != "https://noop.example.org/noop" {
		t.Fatalf("expected photo to be uploaded url, got %+v", got)
	}

	if _, hasVideo := doc.Properties["video"]; hasVideo {
		t.Fatalf("did not expect video property when uploading a photo: %+v", doc.Properties["video"])
	}
}

func TestMicropub_MultipartCreateWithMultipleFiles(t *testing.T) {
	st, cleanup := newIntegrationState(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.Handle("POST /", withToken(st.Cfg, post.DispatchPost(st)))
	mux.Handle("GET /", withToken(st.Cfg, get.DispatchGet(st)))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "Multi File Title")
	_ = writer.WriteField("content", "Multi File Body")

	photoWriter, err := writer.CreateFormFile("photo", "pic.jpg")
	if err != nil {
		t.Fatalf("failed to create photo form file: %v", err)
	}
	photoWriter.Write([]byte("jpg-bytes"))

	videoHeader := textproto.MIMEHeader{}
	videoHeader.Set("Content-Disposition", `form-data; name="video"; filename="clip.mp4"`)
	videoHeader.Set("Content-Type", "video/mp4")

	videoWriter, err := writer.CreatePart(videoHeader)
	if err != nil {
		t.Fatalf("failed to create video form part: %v", err)
	}
	videoWriter.Write([]byte("video-data"))

	writer.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/", &buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected create status: %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("expected Location header from create")
	}

	sourceURL := srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	srcResp, err := client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	if srcResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected source status: %d", srcResp.StatusCode)
	}

	var doc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode source response: %v", err)
	}

	if got := doc.Properties["photo"]; len(got) == 0 || got[0] != "https://noop.example.org/noop" {
		t.Fatalf("expected photo to be uploaded url, got %+v", got)
	}
	if got := doc.Properties["video"]; len(got) == 0 || got[0] != "https://noop.example.org/noop" {
		t.Fatalf("expected video to be uploaded url, got %+v", got)
	}
}

func TestMicropub_MultipartCreateWithVideo(t *testing.T) {
	st, cleanup := newIntegrationState(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.Handle("POST /", withToken(st.Cfg, post.DispatchPost(st)))
	mux.Handle("GET /", withToken(st.Cfg, get.DispatchGet(st)))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "Multipart Video")
	_ = writer.WriteField("content", "Video Body")

	videoHeader := textproto.MIMEHeader{}
	videoHeader.Set("Content-Disposition", `form-data; name="file"; filename="clip.mp4"`)
	videoHeader.Set("Content-Type", "video/mp4")

	videoWriter, err := writer.CreatePart(videoHeader)
	if err != nil {
		t.Fatalf("failed to create video form part: %v", err)
	}
	if _, err := videoWriter.Write([]byte("video-data")); err != nil {
		t.Fatalf("failed to write video content: %v", err)
	}

	writer.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/", &buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	createResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	createResp.Body.Close()

	if createResp.StatusCode != http.StatusAccepted && createResp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected create status: %d", createResp.StatusCode)
	}

	loc := createResp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("expected Location header from create")
	}

	sourceURL := srv.URL + "/?q=source&url=" + url.QueryEscape(loc)
	srcResp, err := client.Get(sourceURL)
	if err != nil {
		t.Fatalf("source request failed: %v", err)
	}
	defer srcResp.Body.Close()

	if srcResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected source status: %d", srcResp.StatusCode)
	}

	var doc content.ContentObject
	if err := json.NewDecoder(srcResp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode source response: %v", err)
	}

	if got := doc.Properties["video"]; len(got) == 0 || got[0] != "https://noop.example.org/noop" {
		t.Fatalf("expected video to be uploaded url, got %+v", got)
	}

	if _, hasPhoto := doc.Properties["photo"]; hasPhoto {
		t.Fatalf("did not expect photo property when uploading a video: %+v", doc.Properties["photo"])
	}
}
