# Dependency updates

## Policy

- Prefer **minimal, intentional** bumps; avoid drive-by major upgrades mid-release.
- CI gates: `go vet`, `go test ./...`, `govulncheck`, frontend `tsc`/build, `npm audit --audit-level=high`.
- Review changelogs for Wails, ytdlp, and webview-related transitive deps carefully.

## Go modules

```bash
go list -m all
go get module@newversion
go mod tidy
make vet test govulncheck
```

Pin `github.com/ytget/ytdlp/v2` explicitly. Remove any `replace` once upstream tags are on the module proxy (see [RELEASING.md](./RELEASING.md)).

## Frontend (npm)

```bash
cd frontend
npm outdated
npm update
npm audit --audit-level=high
npm run build
```

Lockfile (`package-lock.json`) must be committed.

## Cadence

- Security advisories: as needed (same week).
- Routine refresh: before each minor release, or monthly if idle.
- Document notable bumps in `CHANGELOG.md`.
