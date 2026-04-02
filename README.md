# how-i-want-to-code

Minimal Go harness for repeatable Codex dispatch against a single directory in a target repository.

## What It Does

For each run, the harness performs this flow:

1. Validate local tooling: `git`, `gh`, `codex`
2. Validate GitHub auth: `gh auth status`
3. Create isolated run workspace: `/dev/shm/<guid>` (fallback `/tmp/<guid>`)
4. Clone target repo + base branch
5. Run Codex in exactly one configured subdirectory
6. If changes exist: commit, push, and create PR (always the final step)

If Codex fails, no PR is created and the workspace path is printed for inspection.

## Requirements

- Go 1.24+
- `git` on `PATH`
- `gh` on `PATH`, already authenticated (`gh auth login`)
- `codex` on `PATH`

## Build

```bash
go build -o bin/harness ./cmd/harness
```

## Run

Use the template at [`templates/run.example.json`](templates/run.example.json):

```bash
./bin/harness run --config templates/run.example.json
```

## Config (v1)

Required fields:

- `version` (must be `v1`)
- `repo_url`
- `base_branch`
- `target_subdir` (non-root relative path)
- `prompt`
- `commit_message`
- `pr_title`
- `pr_body`

Optional fields:

- `labels` (`[]string`)
- `reviewers` (`[]string`)

## Exit Codes

- `0`: success (PR created or no changes)
- `2`: usage error
- `10`: config error
- `20`: preflight/tooling error
- `21`: auth error
- `30`: workspace error
- `40`: clone error
- `50`: codex execution error
- `60`: git workflow error
- `70`: PR creation error

## Test

```bash
go test ./...
```
