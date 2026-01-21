package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-playground/validator/v10"
)

func validConfig() *Config {
	return &Config{
		Debug: true,
		Server: Server{
			Address:   "127.0.0.1",
			Port:      8080,
			PublicUrl: "https://example.org",
			Limits: ServerLimits{
				MaxPayloadSize:  1,
				MaxFileSize:     1,
				MaxMultipartMem: 1,
			},
		},
		Micropub: Micropub{
			MeUrl:         "https://example.org",
			TokenEndpoint: "https://example.org/token",
		},
		Content: Content{
			Strategy: "git",
			Git: &GitContentStrategy{
				Repository: "https://example.org/repo.git",
				Path:       "content",
				PublicUrl:  "https://example.org/content",
				Auth: GitContentStrategyAuth{
					Method: "plain",
					Plain: &UsernamePasswordAuth{
						Username: "user",
						Password: "pass",
					},
				},
			},
		},
		Media: Media{
			Strategy: "s3",
			S3: &S3MediaStrategy{
				AccessKeyId: "key",
				SecretKeyId: "secret",
				Region:      "us-east-1",
				Bucket:      "bucket",
				Endpoint:    "https://s3.example.com",
				PublicUrl:   "https://cdn.example.com",
			},
		},
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := validConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
}

func TestValidate_FailsForInvalidLocalPath(t *testing.T) {
	cfg := validConfig()
	cfg.Content.Git.Path = "/absolute/path"

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation to fail for absolute path")
	}
}

func TestLoadConfig_Success(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yml")

	yaml := `debug: true
server:
  address: "127.0.0.1"
  port: 8080
  public_url: "https://example.org"
  limits:
    max_payload_size: 1
    max_file_size: 1
    max_multipart_mem: 1
micropub:
  me_url: "https://example.org"
  token_endpoint: "https://example.org/token"
content:
  strategy: "git"
  git:
    repository: "https://example.org/repo.git"
    path: "content"
    public_url: "https://example.org/content"
    auth:
      method: "plain"
      plain:
        username: "user"
        password: "pass"
media:
  strategy: "s3"
  s3:
    access_key_id: "key"
    secret_key_id: "secret"
    region: "us-east-1"
    bucket: "bucket"
    endpoint: "https://s3.example.com"
    public_url: "https://cdn.example.com"
`

	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected config to load, got %v", err)
	}

	if cfg.Server.PublicUrl != "https://example.org" {
		t.Fatalf("unexpected public url: %q", cfg.Server.PublicUrl)
	}
	if cfg.Content.Git == nil || cfg.Content.Git.Path != "content" {
		t.Fatalf("unexpected content path: %+v", cfg.Content.Git)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	if _, err := LoadConfig("/nonexistent/config.yml"); err == nil {
		t.Fatalf("expected error when config file is missing")
	}
}

func TestValidate_FilesystemPathPatternTraversal(t *testing.T) {
	cfg := validConfig()
	cfg.Content.Strategy = "filesystem"
	cfg.Content.Filesystem = &FilesystemContentStrategy{
		Path:        "/tmp/content",
		PublicUrl:   "https://example.org/content",
		PathPattern: "../etc/passwd", // Path traversal attempt
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation to fail for path traversal pattern")
	}
}

func TestValidate_FilesystemPathPatternAbsolute(t *testing.T) {
	cfg := validConfig()
	cfg.Media.Strategy = "filesystem"
	cfg.Media.Filesystem = &FilesystemMediaStrategy{
		Path:        "/tmp/media",
		PublicUrl:   "https://example.org/media",
		PathPattern: "/etc/passwd", // Absolute path attempt
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation to fail for absolute path pattern")
	}
}

func TestValidate_FilesystemValidPathPattern(t *testing.T) {
	cfg := validConfig()
	cfg.Content.Strategy = "filesystem"
	cfg.Content.Filesystem = &FilesystemContentStrategy{
		Path:        "/tmp/content",
		PublicUrl:   "https://example.org/content",
		PathPattern: "{year}/{month}/{slug}.json", // Valid pattern
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation to pass for valid path pattern, got: %v", err)
	}
}

func TestValidate_GitSshWithEmptyPassphrase(t *testing.T) {
	cfg := validConfig()
	cfg.Content.Strategy = "git"
	cfg.Content.Git.Auth.Method = "ssh"
	cfg.Content.Git.Auth.Ssh = &SshKeyAuth{
		Username:           "git",
		PrivateKeyFilePath: filepath.Join(t.TempDir(), "key.pem"),
		Passphrase:         "", // Empty passphrase for unencrypted key
	}
	// Create dummy key file
	os.WriteFile(cfg.Content.Git.Auth.Ssh.PrivateKeyFilePath, []byte("dummy"), 0600)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation to pass for SSH with empty passphrase (unencrypted key), got: %v", err)
	}
}

func TestCustomValidators(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterValidation("abspath", ValidateAbsPath)
	v.RegisterValidation("localpath", ValidateLocalpath)
	v.RegisterValidation("pathpattern", ValidatePathPattern)

	type sample struct {
		Abs     string `validate:"abspath"`
		Local   string `validate:"localpath"`
		Pattern string `validate:"pathpattern"`
	}

	abs := filepath.Join(t.TempDir(), "file.txt")

	if err := v.Struct(sample{Abs: abs, Local: "relative/path", Pattern: "{year}/{month}/{slug}.json"}); err != nil {
		t.Fatalf("expected validator to accept paths: %v", err)
	}

	if err := v.Struct(sample{Abs: "relative", Local: "/abs", Pattern: "valid/pattern"}); err == nil {
		t.Fatalf("expected validator to reject invalid paths")
	}
}

func TestValidatePathPattern(t *testing.T) {
	v := validator.New()
	v.RegisterValidation("pathpattern", ValidatePathPattern)

	type testStruct struct {
		Pattern string `validate:"pathpattern"`
	}

	tests := []struct {
		name    string
		pattern string
		valid   bool
	}{
		{"empty pattern", "", true},
		{"simple pattern", "{slug}.json", true},
		{"nested pattern", "{year}/{month}/{slug}.json", true},
		{"with filename", "{year}/{month}/{filename}", true},
		{"path traversal with ..", "../etc/passwd", false},
		{"path traversal in middle", "posts/../config", false},
		{"absolute unix path", "/etc/passwd", false},
		{"absolute windows path", "C:/Windows", false},
		{"null byte", "posts/\x00evil", false},
		{"complex valid pattern", "{year}/{month}/{day}/{slug}{ext}", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Struct(testStruct{Pattern: tc.pattern})
			if tc.valid && err != nil {
				t.Errorf("expected pattern %q to be valid, got error: %v", tc.pattern, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("expected pattern %q to be invalid, but validation passed", tc.pattern)
			}
		})
	}
}
