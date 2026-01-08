scribble
========

**Scribble does not currently work. It is a brand new project that is under heavy development in my free time. Please do not attempt to use it yet.**

A no-nonsense [micropub](https://indieweb.org/Micropub) server written in Go.

The plan for this is that it is essentially going to be an open pipe that accepts micropub data from clients and pipes it into your desired storage backend. 

That backend may be, for example:
* local to scribble
* a (remote?) git repository (like an Astro site, which will rebuild upon receipt)
* an sftp server
* another http server (maybe)
* some s3 compatible storage
* a database
* something else??

Point being, I'm trying to design it to be agnostic and extensible. It should not care what your content is, its job is to accept it and store it somewhere so you can have it for your own purposes.