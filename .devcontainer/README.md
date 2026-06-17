# Devcontainer for gascity

This devcontainer reproduces a development environment for [gastownhall/gascity](https://github.com/gastownhall/gascity) per the [official installation guide](https://github.com/gastownhall/gascity/blob/main/docs/getting-started/installation.md).

## What it installs

| Tool | Source | Version |
|---|---|---|
| Go 1.26 | `mcr.microsoft.com/devcontainers/go` base image | 1.26 (matches `go.mod`) |
| `tmux` | apt | system |
| `jq` | apt | system |
| `flock` | apt | system (via `util-linux`) |
| `git` | devcontainer feature | latest |
| `gh` | devcontainer feature | latest |
| `dolt` | GitHub release tarball | `DOLT_VERSION` from `deps.env` (currently 2.1.7) |
| `bd` (Beads CLI) | GitHub release tarball | `BD_VERSION` from `deps.env` (currently v1.0.5) |
| `gc` (Gas City) | `make install` from source | built from current commit |

Versions come from `deps.env` so bumping is one file change.

## Lifecycle

| Hook | Runs | Notes |
|---|---|---|
| `onCreateCommand` | Once on container create | `apt install` of the system packages |
| `postCreateCommand` | Once after `onCreate` | Installs `dolt`, `bd`, then builds and installs `gc` from source |
| `postStartCommand` | Every time the container starts | Smoke check that all binaries are on PATH |

## Why source build, not Homebrew

The [installation guide](https://github.com/gastownhall/gascity/blob/main/docs/getting-started/installation.md) recommends Homebrew for daily use. The devcontainer uses the source-build path because:

1. The devcontainer is for contributors and reviewers — `make install` from source is the path documented in "Build from source" and "Contributor setup"
2. Homebrew is not available in the base `devcontainers/go` Ubuntu image
3. Source build matches what CI does in `.github/actions/setup-gascity-ubuntu/`

## Dolt data persistence

`mounts` declares a named volume `gc-dolt-data` at `/home/vscode/.dolt-data` so Dolt databases created by `gc init` survive container rebuilds. Without this, every `postCreateCommand` would start from an empty Dolt state.

## What this does NOT install

- `claude` (Claude Code CLI) — not required for gascity itself, only for agent runtime providers. Install manually if you `gc sling claude`.
- Other agent CLIs (codex, gemini, etc.) — same as above.
- Homebrew — not present in Linux devcontainer base image.

## Testing the devcontainer

```bash
# From repo root, with the devcontainer CLI installed:
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . bash -c 'gc version && gc init /tmp/test-city && cd /tmp/test-city && gc rig add /tmp/test-rig && gc sling claude "echo hello"'
```

Or in VS Code: `Ctrl+Shift+P` → "Dev Containers: Reopen in Container".

## After `gc init`

Follow the [Quickstart](https://github.com/gastownhall/gascity/blob/main/docs/getting-started/quickstart.md):

```bash
gc init ~/my-city
cd ~/my-city
mkdir ~/hello-world && cd ~/hello-world && git init && cd -
gc rig add ~/hello-world
cd ~/hello-world
gc sling claude "Create a script that prints hello world"
bd show <bead-id> --watch
```

## Note on `gc` and Oh My Zsh

If your shell aliases `gc` to `git commit --verbose`, use `command gc ...` to bypass it. The base image here uses bash by default so this is not a problem, but if you opt into Oh My Zsh you'll need the workaround from the [troubleshooting guide](https://github.com/gastownhall/gascity/blob/main/docs/getting-started/troubleshooting.md#oh-my-zsh-git-plugin-hides-gc).
