# Working with OneFit-Engine-plot

Shared guidance for any coding agent working in this repo (Claude Code,
Codex, or otherwise); `CLAUDE.md` points here. No environment-specific
facts belong in this file.

## What this is

`plot-go`: a batch renderer for the Grace project files (`.agr`,
`.agr-par`) OneFit writes, replacing the `gracebat` -> EPS -> `epstopdf`
chain. See README.md for the plan and docs/features.md for exactly which
part of the format is needed.

## Clean-room rule - read this first

This repo is Artistic License 2.0, and part of the suite's dual-license
(Artistic or commercial) policy. Grace is GPL. Therefore:

- Never read, copy, or paraphrase Grace's source code (its `src/`, `T1lib/`,
  `cephes/`, `Xbae/` directories, or Debian patches to them), and never
  download or unpack it where you might read it.
- Do use the file format, the Grace User's Guide and man pages (`doc/` in
  the Grace source tarball, or online), and `gracebat` as a black box:
  run it and compare its output.
- Do not copy the User's Guide's text into this repo; describe in your own
  words.

## Before changing anything

1. `git status`.
2. `go vet ./...` and `go test ./...`.
3. Keep `docs/features.md` in step with what the renderer supports.

## Conventions

- Go, standard library first; any dependency needs a license compatible
  with Artistic 2.0 and the commercial option (BSD, MIT, Apache) - no GPL.
- The renderer must accept every file in the corpus: an unknown or
  malformed directive is a warning, not an error, as in gracebat.
- Tests compare against gracebat output within a tolerance; never edit a
  reference image to make a test pass without saying why in the commit.
