## Project Info

Belochka (белочка, "squirrel") — a single-binary Go+React web app for managing 5-20 remote Linux servers via persistent SSH connections. Streams CPU, memory, disk, network, and process metrics to a browser dashboard via WebSocket, and provides a web-based interactive terminal (SSH console) for direct server access.

## Repo State

- **Location**: `/home/kilian/code/belochka`
- **Branch**: `main`

## Constraints

- Always reply in simplified Chinese, regardless of user's input language.
- Write or modify files only in English.
- When commit, do not mention *Co-Authored-By: Claude* in the commit message

## Requirements

- Before any code exploration or editing, **always** read `CODEBASE.md` first to understand the project structure, components, data flows, and external dependencies.
- Based on `CODEBASE.md` and the current task, determine the exact set of files that are relevant. **Only** read and modify those files; never load or touch unrelated files. This keeps the context clean and focused.
- After modifying code, **proactively** update any associated code (imports, references, configurations, etc.) to eliminate zombie code, broken references, or incompatibilities introduced by the change.

## Guidelines

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## Future Consideration

- Public internet deployment
- SSH key passphrase support
- User-configurable card ordering (drag and drop)
- Docker-native deployment (Dockerfile)