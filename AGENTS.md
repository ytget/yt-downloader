# AGENTS.md

Instructions for coding agents working in this repository.

## Stability backlog (required reading)

Unfinished product-stability work lives here:

- **`docs/STABILITY_TASKS.md`**

That file is the source of truth for remaining implementation work toward a stable application.

### On every new agent run

1. Read `docs/STABILITY_TASKS.md`.
2. Determine which tasks are still open (`[ ]` or `[~]`).
3. **Default behavior when the user does not give a different task:**
   - Continue the highest-priority unfinished item (Phase A → B → C → D → E → F → G).
   - Prefer one coherent slice; mark `[~]` while in progress and `[x]` when done.
   - Update the Progress log in `docs/STABILITY_TASKS.md` when completing items.
4. **If the user asks something else** (bugfix, question, feature, refactor, explanation):
   - Do **that** request first.
   - Do not force backlog work in the same turn unless they also ask to continue the list.
5. **If all tasks in `docs/STABILITY_TASKS.md` are `[x]`:**
   - Do not invent new backlog work automatically.
   - Answer the user’s question or ask what they want next.
   - Optionally mention that the stability backlog is complete.

### Refactoring handoff

When the user asks to continue the code-simplification/refactoring work, or when this work is already explicitly in progress, read `docs/REFACTORING_TASKS.md` and continue the first unfinished `R*` task in order. Follow that file's new-agent instructions, update its checklist/progress log, and keep the repository buildable after each task.

This refactoring instruction does not override a new, unrelated user request. The user may always:

- ask questions unrelated to the backlog
- pause backlog work
- reprioritize (“do C2 next”, “skip packaging”)
- add/change tasks in `docs/STABILITY_TASKS.md`

User instructions always win over this default backlog behavior.

## Project context (short)

- Desktop app: **Wails v2** + Go backend + Vite/TS frontend.
- Downloads/captions engine: **`github.com/ytget/ytdlp/v2`** (may be locally replaced until published).
- Transcripts: YouTube captions → `.srt` + `.txt` via `internal/transcript`.
- Settings: JSON under user config dir (`internal/config`).

## Practical commands

```bash
make deps
make run          # wails dev
make test
make build
```

## Working rules

- Keep changes scoped; match existing style.
- Prefer tests for lifecycle, captions parsing, and settings.
- Do not commit secrets.
- Do not implement deferred items (ASR, mobile, etc.) unless explicitly requested.
- When finishing a stability task, leave the repo buildable (`go test ./...`, frontend build as relevant).
