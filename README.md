scribble
========

A no-nonsense [micropub](https://indieweb.org/Micropub) server written in Go.

Current status
--------------
- Full Micropub server implementation with spec-compliant updates (slug changes return HTTP 201)
- Multiple content storage backends: Git, PostgreSQL/MySQL, Cloudflare D1, Filesystem
- Multiple media storage backends: S3-compatible (S3, R2, MinIO, etc.), Filesystem
- Collision-safe updates with UUID-based conflict resolution
- Transaction-based writes (SQL) and batch execution (D1) for data integrity
- Flexible path patterns for organizing files by date and custom structures
- More features are planned; expect breaking changes while things stabilize.

Goal
----
Scribble is an open pipe for Micropub: accept incoming posts and push them to whatever storage backend you choose.

Quick Start
-----------

1. Copy the default configuration:
   ```bash
   cp config.default.yml config.yml
   ```

2. Edit `config.yml` with your settings (see [CONFIGURATION.md](CONFIGURATION.md) for details)

3. Run the server:
   ```bash
   go run ./cmd/scribble -config config.yml
   ```
   
   Or build and run:
   ```bash
   go build -o scribble ./cmd/scribble
   ./scribble -config config.yml
   ```

Configuration
-------------

See [CONFIGURATION.md](CONFIGURATION.md) for comprehensive configuration documentation including:
- Server and Micropub settings
- All content storage backends (Git, SQL, D1, Filesystem)
- All media storage backends (S3-compatible, Filesystem)
- Path patterns for flexible file organization
- Security considerations and validation rules

Content Storage Backends
------------------------
- **Git repository** - Store posts in a Git repo (perfect for static site generators)
- **PostgreSQL/MySQL** - Store in a relational database
- **Cloudflare D1** - Serverless edge database
- **Local filesystem** - Simple file-based storage with flexible path patterns
- ~~SFTP server~~ (planned)
- ~~HTTP forwarding~~ (planned)

Media Storage Backends
----------------------
- **S3-compatible** - AWS S3, Cloudflare R2, MinIO, Backblaze B2, etc.
- **Local filesystem** - Store files locally with date-based organization
- ~~SFTP server~~ (planned)