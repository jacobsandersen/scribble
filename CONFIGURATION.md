# Configuration Guide

Scribble is configured via a YAML file (default: `config.yml`). You can specify a custom path with the `-config` flag:

```bash
./scribble -config /path/to/config.yml
```

## Quick Start

Copy the default configuration and customize it:

```bash
cp config.default.yml config.yml
```

## Configuration Structure

### Server Settings

```yaml
server:
  address: "0.0.0.0"           # Bind address (use 127.0.0.1 for local-only)
  port: 9000                    # Port to listen on
  public_url: "https://example.org"  # Your public-facing URL
  limits:
    max_payload_size: 2000000   # Max request body size (bytes)
    max_file_size: 10000000     # Max media upload size (bytes)
    max_multipart_mem: 20000000 # Memory buffer for multipart parsing
```

### Micropub Settings

```yaml
micropub:
  me_url: "https://example.org"  # Your domain/identity URL
  token_endpoint: "https://tokens.indieauth.com/token"  # Token verification endpoint
```

Scribble validates all incoming requests against the configured `token_endpoint`. For IndieAuth, use `https://tokens.indieauth.com/token`. For custom authorization servers, point to your own token endpoint.

## Content Storage Backends

Scribble stores Micropub posts as JSON documents in various backends. Choose one strategy:

### Git Backend

Stores content in a Git repository (perfect for static site generators):

```yaml
content:
  strategy: git
  git:
    repository: "https://github.com/username/repo.git"
    path: "content/posts"        # Subdirectory within repo
    public_url: "https://example.org/posts/"
    auth:
      method: plain              # or "ssh"
      plain:
        username: "your-username"
        password: "your-token-or-password"
```

**SSH Authentication:**

```yaml
      method: ssh
      ssh:
        username: "git"
        private_key_file_path: "/home/user/.ssh/id_ed25519"
        passphrase: ""  # Optional, leave empty for unencrypted keys
```

**How it works:**
- Clones the repository on startup
- Writes posts as `<slug>.json` files
- Commits and pushes changes immediately
- Uses mutex locks to prevent conflicts

### SQL Backend (PostgreSQL/MySQL)

Stores content in a relational database:

```yaml
content:
  strategy: sql
  sql:
    driver: postgres  # or "mysql"
    dsn: "postgres://user:pass@localhost:5432/scribble?sslmode=disable"
    public_url: "https://example.org/posts/"
    table_prefix: "scribble"  # Optional, defaults to "scribble"
```

**MySQL DSN example:**
```
user:password@tcp(localhost:3306)/scribble?parseTime=true
```

**How it works:**
- Creates table `<prefix>_content` automatically
- Stores JSON documents with slug indexing
- Uses transactions for atomic updates
- Slug changes use DELETE+INSERT for consistency

### Cloudflare D1 Backend

Stores content in Cloudflare's serverless D1 database:

```yaml
content:
  strategy: d1
  d1:
    account_id: "your-account-id"
    database_id: "your-database-id"
    api_token: "your-cloudflare-api-token"
    public_url: "https://example.org/posts/"
    table_prefix: "scribble"  # Optional
    endpoint: ""  # Optional, for custom API endpoints
```

**How it works:**
- Uses Cloudflare D1 HTTP API
- Same schema as SQL backend
- Sequential batch execution for updates (D1 limitation)
- Automatic schema initialization

### Filesystem Backend

Stores content as JSON files on the local filesystem:

```yaml
content:
  strategy: filesystem
  filesystem:
    path: "/var/www/scribble/content"  # Must be absolute path
    public_url: "https://example.org/posts/"
    path_pattern: "{slug}.json"  # Optional, see Path Patterns below
```

**Path Patterns:**

Customize how files are organized using placeholders:

- `{slug}` - Post slug
- `{year}` - 4-digit year (e.g., "2026")
- `{month}` - 2-digit month (e.g., "01")
- `{day}` - 2-digit day (e.g., "15")
- `{ext}` - File extension with dot (e.g., ".json")
- `{filename}` - Complete filename with extension

**Examples:**
```yaml
path_pattern: "{slug}.json"                    # Flat: my-post.json
path_pattern: "{year}/{month}/{slug}.json"     # Dated: 2026/01/my-post.json
path_pattern: "{year}/{slug}{ext}"             # Year only: 2026/my-post.json
```

**How it works:**
- Maintains in-memory slug-to-path index
- Rebuilds index on startup by scanning directory
- Atomic file writes with conflict detection
- Supports hierarchical organization via patterns

## Media Storage Backends

Scribble stores uploaded media files separately from content.

### S3-Compatible Backend

Works with AWS S3, Cloudflare R2, MinIO, Backblaze B2, and other S3-compatible services:

```yaml
media:
  strategy: s3
  s3:
    access_key_id: "your-access-key"
    secret_key_id: "your-secret-key"
    region: "us-east-1"          # AWS region or equivalent
    bucket: "my-media-bucket"
    endpoint: ""                 # Optional: custom endpoint (R2, MinIO, etc.)
    force_path_style: false      # Set true for MinIO and some providers
    disable_ssl: false           # Set true for local development only
    prefix: "media/"             # Optional: prefix for all uploads
    public_url: ""               # Optional: CDN or custom domain
```

**Cloudflare R2 example:**
```yaml
    endpoint: "https://[account-id].r2.cloudflarestorage.com"
    region: "auto"
    public_url: "https://cdn.example.org"
```

**MinIO example:**
```yaml
    endpoint: "http://localhost:9000"
    force_path_style: true
    disable_ssl: true
```

**How it works:**
- Validates bucket exists on startup
- Uploads with unique filenames to prevent collisions
- Returns public URL based on `public_url` or endpoint
- Uses standard S3 API for compatibility

### Filesystem Backend

Stores media files on the local filesystem:

```yaml
media:
  strategy: filesystem
  filesystem:
    path: "/var/www/scribble/media"  # Must be absolute path
    public_url: "https://example.org/media/"
    path_pattern: "{year}/{month}/{filename}"  # Optional, see Path Patterns
```

**Path Patterns:**

Same placeholders as content patterns, plus:
- `{filename}` - Original uploaded filename
- Media uploads use current timestamp for `{year}`, `{month}`, `{day}`

**Examples:**
```yaml
path_pattern: "{filename}"                       # Flat: photo.jpg
path_pattern: "{year}/{month}/{filename}"        # Dated: 2026/01/photo.jpg
path_pattern: "{year}/{month}/{day}/{filename}"  # Daily: 2026/01/21/photo.jpg
```

**How it works:**
- Creates directories as needed based on pattern
- Detects filename collisions and appends UUID if needed
- Supports any file type with MIME detection
- Returns public URL by joining `public_url` + relative path

### Noop Backend (Testing)

Silently accepts uploads but doesn't store anything:

```yaml
media:
  strategy: noop
```

Useful for development or when media isn't needed.

## Path Pattern Security

Path patterns are validated to prevent security issues:

- ❌ Cannot contain `..` (path traversal)
- ❌ Cannot be absolute paths (`/`, `C:/`)
- ❌ Cannot contain null bytes
- ✅ Must be relative paths only

Invalid patterns will fail validation at startup.

## Configuration Validation

Scribble validates your configuration on startup:

- Required fields must be present
- URLs must be valid
- File paths must be absolute where required
- Table prefixes must be valid identifiers
- Path patterns must pass security checks

If validation fails, Scribble will exit with a descriptive error.

## Environment-Specific Configs

Run different configurations per environment:

```bash
# Development
./scribble -config config.dev.yml

# Production
./scribble -config config.prod.yml
```

## Example: Complete Configuration

See [`config.default.yml`](config.default.yml) for a complete, commented example with all options.

## Troubleshooting

**"unknown content strategy"** - Check the `strategy` field matches exactly: `git`, `sql`, `d1`, or `filesystem`

**"filesystem content config is nil"** - Ensure the strategy-specific config block (e.g., `filesystem:`) is present

**"failed to build index"** - For filesystem backend, ensure the path exists and is readable

**"bucket does not exist"** - For S3 backend, verify bucket name and credentials are correct

**"validation failed"** - Check the error message for which field failed validation and consult the examples above
