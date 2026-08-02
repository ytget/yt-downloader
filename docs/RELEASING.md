# Releasing

## Versioning

- **SemVer** source of truth: root `VERSION` file (e.g. `1.0.0`)
- `make sync-version` writes `productVersion` into `wails.json`
- Binaries embed version via `-ldflags "-X main.version=..."` (see `Makefile`)
- Git tags: `vMAJOR.MINOR.PATCH` (leading `v`)

## Pre-release checklist

1. [ ] `VERSION` bumped; `CHANGELOG.md` updated
2. [ ] `make check` (fmt, vet, tests, frontend typecheck)
3. [ ] `make govulncheck` and `make npm-audit` clean for policy
4. [ ] Captions/ytdlp pin reviewed (`go.mod` `replace` only if still required)
5. [ ] `docs/QUALITY.md` / user-facing notes still accurate
6. [ ] Package smoke: `make build` (or CI `package-smoke`) on target OS
7. [ ] Signing secrets available for macOS/Windows if publishing signed builds ([PACKAGING.md](./PACKAGING.md))

## ytdlp bump process

1. Land/verify the captions API on `github.com/ytget/ytdlp` (or temporary fork).
2. Tag a `v2.x.y` release consumed by the module proxy.
3. While upstream is blocked, use the pinned fork replacement in `go.mod`:
   ```text
   replace github.com/ytget/ytdlp/v2 => github.com/bangoo2bangoo/ytdlp/v2 v2.1.0
   ```
4. Once upstream publishes the tag, run:
   ```bash
   go get github.com/ytget/ytdlp/v2@v2.x.y
   # Remove the fork replacement from go.mod.
   go mod tidy
   go test ./...
   ```
5. Note the bump in `CHANGELOG.md`.

## Tag and publish

```bash
# after merge to main
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GitHub Actions `release.yaml` builds artifacts, optionally signs, and attaches release assets. Homebrew cask points at the published `yt-downloader.app.zip` (see [PACKAGING.md](./PACKAGING.md)).

## Post-release

1. Verify release assets download and launch on each OS.
2. Update cask `sha256` when leaving `:no_check`.
3. Confirm support bundle shows the new version.
