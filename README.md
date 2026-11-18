# agent-relay

Cross-OS session synchronizer, rate-limit recovery tool, and workspace handoff utility for AI coding assistants.

## Overview

`agent-relay` bridges developer environments when working across multiple computers and operating systems (such as a Windows desktop and an Arch Linux laptop). When an AI coding session encounters a sliding rate limit or token quota ceiling, `agent-relay` inspects, extracts, and translates the full conversational state along with uncommitted Git modifications so you can resume work on another machine without loss of context.

## Core Capabilities

- **Automatic Harness Detection**: Scans and indexes local sessions across multiple agent environments:
  - **Claude Code** (`~/.claude/projects/`, memories, file history snapshots, transcripts).
  - **Google Antigravity** (`~/.gemini/antigravity/brain/`, workspaces, system logs).
  - **Cursor IDE** (workspace storage, composer databases, and session state).
  - **Codeium Windsurf** (Cascade agent workspaces and memories).
  - **Cline / Roo Code** (autonomous task history and conversation JSON).
  - **Aider** (repository-level chat logs and command history).
- **Quota Tracking**: Identifies sessions halted by usage limits and displays the calculated reset time.
- **Cross-Platform Path Translation**: Automatically rewrites absolute and escaped file paths between Windows, Linux, and macOS.
- **Workspace State Preservation**: Captures uncommitted changes (`git diff HEAD`) and untracked files into the relay bundle to prevent codebase desynchronization.
- **Terminal User Interface**: Interactive terminal interface written in Go with Bubble Tea and Lip Gloss.
- **End-to-End Encrypted Relay**: Share sessions between separate networks using client-side AES-256-GCM encryption paired with a single-use code.
- **Offline Bundle Archives**: Generates `.tar.gz` bundles for transfer over local storage or external media.

## Building and Running

Prerequisites: Go 1.22+ and Git.

To compile:

```
go build -o agent-relay cmd/agent-relay/main.go
```

To run the interactive interface:

```
./agent-relay
```

## Interactive Keybindings

- `Up` / `Down` or `k` / `j`: Move selection through projects or conversations.
- `Enter`: Open conversation list (from project view) or export session bundle (from conversation view).
- `Tab`: Switch filter between all harnesses, Claude Code, and Antigravity.
- `/`: Filter project list in real time by name or path.
- `s`: Encrypt current session using AES-256-GCM and generate a network transfer code.
- `g`: Download and decrypt a session bundle using a transfer code.
- `i`: Open local import browser to restore a `.tar.gz` bundle.
- `r`: Rescan and refresh sessions from disk.
- `Esc` / `Backspace`: Return to project browser or exit current prompt.
- `q`: Exit agent-relay.

## How Transfers Work

1. Export: The adapter gathers transcript JSONL data, tool execution outputs, project memory, file snapshots, and Git working tree diffs into a structured archive.
2. Path Normalization: During import on the target operating system, paths (including Windows backslashes, drive prefixes, and Git-Bash POSIX references) are converted to match the destination filesystem layout.
3. Restore: The session is placed into the destination harness directory and Git modifications are applied to the local repository.
