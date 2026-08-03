## Project Info

Belochka (белочка, "squirrel") — a single-binary Go+React web app for managing 5-20 remote Linux servers via persistent SSH connections. Streams CPU, memory, disk, network, and process metrics to a browser dashboard via WebSocket, and provides a web-based interactive terminal (SSH console) for direct server access.

## Constraints

- Always reply in simplified Chinese, regardless of user's input language.
- Write or modify files only in English.
- When commit, do not mention *Co-Authored-By: Claude* in the commit message

## Requirements

- Before any code exploration or editing, **always** read `CONTEXT.md` first to internalize the domain vocabulary (Server, Snapshot, Collector, Pool, Hub, etc.) and understand the architectural decisions that shaped the codebase.
- Use the terms defined in `CONTEXT.md` consistently. Don't invent synonyms — if you need a concept that isn't there, flag it rather than drifting the language.
- If a proposed change contradicts an architectural decision recorded in `CONTEXT.md`, surface it explicitly rather than silently overriding.
- Determine relevant files by exploring the directory structure and using grep — `CONTEXT.md` gives you the concepts, not the file map. **Only** read and modify files that are actually relevant to the task.
- After modifying code, **proactively** update any associated code (imports, references, configurations, etc.) to eliminate zombie code, broken references, or incompatibilities introduced by the change.
- When reporting information to me, be extremely concise and sacrifice grammar for the sake of concision.

## Agent skills

### Issue tracker

Issues live as local markdown files under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.