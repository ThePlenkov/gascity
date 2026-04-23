---
title: Session Environment
description: Environment variables gc sets inside agent sessions, including the BEADS_ACTOR convention.
---

# Session Environment

`gc` populates a fixed set of environment variables in every agent
session it spawns. Pack authors should rely on these keys in prompt
templates and hook scripts rather than re-deriving identity from
process metadata or session state files.

The key to know is `$GC_TEMPLATE`, which identifies the configured
agent and is stable across every spawn path (named singleton,
pool/ephemeral, manual). Prompt templates should route and claim work
through `$GC_TEMPLATE`.

## BEADS_ACTOR convention

`BEADS_ACTOR` is the default actor used by `bd` mutations, including
`bd update <id> --claim` when `--assignee` is omitted. Its value
depends on how the session was spawned:

| Session origin | `BEADS_ACTOR` | Example |
|----------------|---------------|---------|
| Named singleton (max_active_sessions = 1) | `$GC_TEMPLATE` — the raw `<rig>/<template>` identity | `gascity/investigator` |
| Pool / ephemeral | `$GC_SESSION_NAME` — `<template-basename>-<bead-id>` | `designer-lgf-b807` |
| Manual (`gc session new` without `--alias`) | `$GC_SESSION_NAME` — `s-<bead-id>` | `s-lgf-jtvr` |

For **named singleton** sessions,
`BEADS_ACTOR == GC_AGENT == GC_TEMPLATE == <rig>/<template>`.

For **pool/ephemeral** and **manual** sessions,
`BEADS_ACTOR == GC_SESSION_NAME == GC_AGENT` and
`GC_TEMPLATE` carries the stable template identity.

`bd update --claim --assignee=<X>` uses `<X>` literally — the
`--assignee` flag overrides the `BEADS_ACTOR` default for that call.
Pack prompts pin `--assignee="$GC_TEMPLATE"` so claims route to the
stable identity regardless of spawn path, which keeps tier-1 recovery
(`bd list --assignee="$GC_TEMPLATE"`) reliable.
