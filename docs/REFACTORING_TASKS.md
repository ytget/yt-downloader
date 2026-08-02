# Code Simplification and Reuse Refactoring Tasks

This document tracks the remaining refactoring work identified during the project review.

## How to use

- Implement one task at a time.
- Keep the repository buildable after every task.
- Add or update tests for behavior that changes.
- Mark a task `[~]` while working and `[x]` only after implementation and verification.
- Do not combine unrelated tasks in one change.

## Instructions for a new coding agent

Before changing code:

1. Read `AGENTS.md`.
2. Read `docs/STABILITY_TASKS.md` as required by the repository instructions.
3. Read this file completely.
4. Inspect the current git diff and do not overwrite unrelated user changes.
5. Select the first unfinished refactoring task in order: `R1`, then `R2`, and so on.

For each task:

1. Mark the task `[~]`.
2. Inspect the relevant implementation and existing tests.
3. Implement only that task's scope.
4. Add or update focused tests.
5. Run `gofmt` for Go changes.
6. Run `go test ./...`.
7. Run `go test -race ./...` when the task affects concurrency or ownership.
8. Run `cd frontend && npm run build` for frontend or binding changes.
9. Fill in the task's `Implementation notes` and `Verification` sections.
10. Mark the task `[x]` only when all acceptance criteria are satisfied.
11. Add a short entry to the progress log.

Important current context:

- This is a Wails v2 desktop application with a Go backend and TypeScript frontend.
- The download implementation is in `internal/download/service.go` and is intentionally being refactored incrementally.
- Preserve existing behavior for pause, resume, stop, restart, transcript retry, playlists, persistence, and partial transcript success.
- Do not implement deferred product features such as ASR, mobile support, browser extensions, or subtitle embedding.
- Avoid broad rewrites. Keep each change buildable and reversible.

## Status

- [x] Runtime service configuration consolidation
- [x] Settings normalization consolidation
- [x] URL utility extraction
- [x] Persistence signal coalescing
- [x] Basic task and playlist snapshots
- [x] Basic task transition helper
- [x] Basic frontend action helper
- [ ] R1. Split the download service into focused files
- [ ] R2. Complete task lifecycle state-machine refactor
- [ ] R3. Isolate callbacks and event delivery from service locks
- [ ] R4. Complete immutable task and playlist boundaries
- [ ] R5. Extract shared media/transcript pipeline operations
- [ ] R6. Unify file-output and collision-policy handling
- [ ] R7. Reduce duplicated DTO and frontend model definitions
- [ ] R8. Decompose the frontend and add event delegation
- [ ] R9. Remove obsolete compatibility code and stabilize concurrency tests

---

## R1. Split the download service into focused files

### Goal

Reduce the size and responsibility of `internal/download/service.go`.

### Scope

Move implementation into focused files while preserving the public `Downloader` API:

- `service.go`: construction, dependencies, public coordination
- `lifecycle.go`: task lifecycle operations
- `pipeline.go`: media and transcript workflow
- `progress.go`: progress, speed, ETA, smoothing
- `playlist.go`: playlist scheduling and child tasks
- `persistence.go`: queue snapshots and restore
- `configuration.go`: runtime configuration
- `errors.go`: user-facing error classification

### Acceptance criteria

- Public behavior remains unchanged.
- `go test ./...` passes.
- No new circular dependencies are introduced.
- Each extracted file has one clear responsibility.

### Implementation notes


### Verification


---

## R2. Complete task lifecycle state-machine refactor

### Goal

Make task state transitions explicit and eliminate repeated field mutation logic.

### Scope

- Define valid task states and transitions.
- Centralize status, phase, timestamps, errors, progress, and transcript reset behavior.
- Use transition helpers for pause, stop, resume, restart, retry, success, and failure.
- Reject invalid transitions consistently.
- Preserve partial-success behavior for media plus transcript downloads.

### Acceptance criteria

- Every lifecycle mutation goes through a transition/helper method.
- Invalid transitions return consistent errors.
- Existing lifecycle tests continue to pass.
- Add table-driven transition tests.

### Implementation notes


### Verification


---

## R3. Isolate callbacks and event delivery from service locks

### Goal

Prevent callbacks, UI events, and persistence work from running while internal service mutexes are held.

### Scope

- Replace direct callback invocation from locked sections with an event channel or publisher.
- Define event payload ownership and ordering rules.
- Coalesce high-frequency progress events where appropriate.
- Ensure shutdown drains or stops event delivery safely.
- Preserve task update and completion notifications.

### Acceptance criteria

- No service callback executes while `tasksMutex`, playlist locks, or configuration locks are held.
- No goroutine is created per progress update.
- Shutdown does not lose terminal task updates.
- Add callback ordering and shutdown tests.

### Implementation notes


### Verification


---

## R4. Complete immutable task and playlist boundaries

### Goal

Ensure callers cannot mutate live worker-owned task or playlist state.

### Scope

- Return detached snapshots from all public read methods.
- Deep-copy playlist videos and other mutable collections.
- Ensure update callbacks receive snapshots.
- Avoid passing live model pointers into asynchronous operations where possible.
- Document ownership rules for internal worker methods.

### Acceptance criteria

- External mutation of returned tasks/playlists cannot affect service state.
- No data races are introduced by public reads.
- Add tests for shallow and nested mutation attempts.
- `go test -race ./internal/download` passes, excluding no tests without documented justification.

### Implementation notes


### Verification


---

## R5. Extract shared media/transcript pipeline operations

### Goal

Remove duplicated orchestration between normal downloads and transcript retry.

### Scope

Create reusable operations for:

- resolving metadata
- downloading media
- fetching captions
- selecting caption language
- writing transcript files
- retry and timeout behavior
- partial-success result handling

`RetryTranscript` should reuse the same transcript operation used by the main pipeline.

### Acceptance criteria

- Media and transcript paths share common retry/error handling.
- Transcript-only, media-plus-transcript, and retry-transcript behavior remain correct.
- Add unit tests using injected media and caption engines.
- No live YouTube dependency is required for core tests.

### Implementation notes


### Verification


---

## R6. Unify file-output and collision-policy handling

### Goal

Provide one reusable output abstraction for media and transcript files.

### Scope

- Centralize output path construction.
- Centralize filename sanitization and extension handling.
- Centralize collision resolution for skip, overwrite, and rename.
- Centralize atomic writing and cleanup of temporary files.
- Return structured output results instead of separate path/skip checks.

### Acceptance criteria

- Media and transcript outputs use the same collision abstraction.
- Partial writes cannot leave corrupt final files.
- Windows reserved names, long names, and Unicode behavior remain covered.
- Add tests for all collision policies and interrupted writes.

### Implementation notes


### Verification


---

## R7. Reduce duplicated DTO and frontend model definitions

### Goal

Avoid maintaining the same task, playlist, settings, and enum definitions in multiple places.

### Scope

- Identify duplicated Go model, DTO, and TypeScript definitions.
- Select a single source of truth.
- Generate or centrally derive frontend wire types where practical.
- Keep frontend-facing DTOs separate from internal-only fields.
- Add compatibility handling for persisted data changes.

### Acceptance criteria

- Task/status/mode/transcript enums cannot silently drift between Go and TypeScript.
- DTO conversion code is smaller and tested.
- Frontend type checking passes.
- Existing Wails bindings remain functional.

### Implementation notes


### Verification


---

## R8. Decompose the frontend and add event delegation

### Goal

Reduce the size and coupling of `frontend/src/main.ts`.

### Scope

Extract focused modules for:

- application state
- shell rendering
- task list and task row rendering
- settings modal
- audio picker modal
- task actions
- formatting and DOM utilities
- notification handling

Use event delegation for task-row actions instead of rebinding every button after rendering.

### Acceptance criteria

- `main.ts` becomes a bootstrap/composition module.
- Progress updates do not rebuild unrelated UI.
- Task actions continue working after incremental updates.
- Scroll position is preserved.
- `npm run build` passes.

### Implementation notes


### Verification


---

## R9. Remove obsolete compatibility code and stabilize concurrency tests

### Goal

Reduce dead code and make concurrency tests deterministic.

### Scope

- Review and remove obsolete compatibility wrappers and unused methods.
- Remove reserved-but-unused code only after confirming no API requirement remains.
- Replace sleeps and polling loops in tests with channels, contexts, or injectable clocks.
- Fix remaining race detector findings.
- Add lifecycle stress coverage for pause, stop, remove, retry, playlist cancel, and shutdown.

### Acceptance criteria

- No unreferenced compatibility code remains without documentation.
- Tests do not depend on arbitrary sleeps where synchronization is possible.
- `go test ./...` passes repeatedly.
- `go test -race ./...` passes.
- Frontend build and relevant CI checks pass.

### Implementation notes


### Verification


---

## Progress log

| Date | Task | Note |
|------|------|------|
|  |  |  |
