scribble
========

A no-nonsense [micropub](https://indieweb.org/Micropub) server written in Go.

Current status
--------------
- Full Micropub server implementation with spec-compliant updates (slug changes return HTTP 201)
- Multiple content storage backends: Git, PostgreSQL/MySQL, Cloudflare D1
- S3-compatible media storage (S3, R2, MinIO, etc.)
- Collision-safe updates with UUID-based conflict resolution
- Transaction-based writes (SQL) and batch execution (D1) for data integrity
- More features are planned; expect breaking changes while things stabilize.

Goal
----
Scribble is an open pipe for Micropub: accept incoming posts and push them to whatever storage backend you choose.

Content Storage Backends
------------------------
- Git repository (e.g., for static-site rebuilds) ✅
- PostgreSQL/MySQL database ✅
- Cloudflare D1 (serverless) ✅
- Local filesystem (planned)
- SFTP server (planned)
- HTTP forwarding (planned)
- Others as they emerge

Media Storage Backends
----------------------
- S3-compatible storage (S3, R2, MinIO, etc.) ✅