scribble
========

A no-nonsense [micropub](https://indieweb.org/Micropub) server written in Go.

Current status
--------------
- Working Micropub server backing a git content store (writes posts to a git repo)
- Working S3-compatible media store (uploads media to S3/R2/etc.)
- More backends and features are planned; expect breaking changes while things stabilize.

Goal
----
Scribble is an open pipe for Micropub: accept incoming posts and push them to whatever storage backend you choose.

Planned/possible backends
-------------------------
- Local filesystem
- Git repository (e.g., for static-site rebuilds) ✅
- SFTP server
- HTTP forwarding
- S3-compatible storage ✅
- Database (PostgreSQL/MySQL) ✅
- Others as they emerge