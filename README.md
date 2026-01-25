scribble
========

A no-nonsense [micropub](https://indieweb.org/Micropub) server written in Go.

Current status
--------------
- Full Micropub server implementation
- Writes content to Cloudflare D1 (more backends planned upon feature completion)
- Writes media to S3-compatible hosts (S3, R2, MinIO, etc.)
- Collision-safe updates with UUID-based conflict resolution
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

2. Edit `config.yml` with your settings.

3. Run the server:
   ```bash
   go run ./cmd/scribble -config config.yml
   ```
   
   Or build and run:
   ```bash
   go build -o scribble ./cmd/scribble
   ./scribble -config config.yml
   ```
