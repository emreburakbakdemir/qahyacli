# qahyacli

A research prototype for Qahya: **how can shell commands in a job change environment variables — including credentials — so that later commands see the change?**

A child process can't change its parent's environment, so `export` inside a step dies with that step. qahyacli works around this the way Buildkite's job API does: the **runner owns the job's env**, steps change it through a CLI that talks to the runner over a private unix socket, and every new step is spawned with a fresh env built from that state.

## Quick start

```sh
go build -o bin/qahyacli ./cmd/qahyacli
./bin/qahyacli run examples/demo.yaml     # a 21-step tour of every feature
sh examples/run_tests.sh                  # all example jobs, PASS/FAIL summary
```

A minimal job:

```yaml
steps:
  - name: set
    run: qahyacli env set GREETING="hello world"
  - name: read
    run: echo "$GREETING"                 # hello world
```

## How it works

```
qahyacli run job.yaml                                (host role)
 ├─ Store            the job's env, in memory
 ├─ job API          HTTP+JSON on /tmp/qahya-XXXX/job.sock
 └─ for each step:
      env = PATH, HOME, QAHYA_JOB_API, QAHYA_STEP + store (secrets read now)
      sh -c "<run>"
        └─ qahyacli env set|get|list|unset|load      (client role)
              └─ socket ─▶ job API ─▶ Store ─▶ seen by the next step
```

It is one binary with two roles. The runner puts its own directory first on each step's `PATH`, so `qahyacli` inside a step is the same binary that is running the job.

Built-in decisions:

- **One shell per step.** A step's own `export` is lost when it exits; changes go through `qahyacli env`.
- **Nothing is inherited.** A step gets only `PATH`, `HOME`, `QAHYA_JOB_API`, `QAHYA_STEP` and the store's contents — not the runner's environment.
- **Secrets: check early, fetch late.** `env load` verifies every secret reference exists; the value itself is read again each time a step starts, so editing the secrets file between steps (a rotation) needs no reload.
- **Loads are all-or-nothing.** A config with one bad entry changes nothing.
- **Errors exit 1.** A rejected `qahyacli env set` fails its step, and a failing step stops the job.

## Commands

| Command | Where | What it does |
|---|---|---|
| `qahyacli run <job.yaml>` | terminal | run a job |
| `qahyacli env set NAME=VALUE...` | step | set variables for **later** steps |
| `qahyacli env get NAME` | step | print one value; works in the same step and resolves secrets |
| `qahyacli env list` | step | table of all variables with their source; secrets shown as references |
| `qahyacli env unset NAME...` | step | remove variables for later steps |
| `qahyacli env load FILE [--profile P] [--export]` | step | load a config for later steps |

`env` commands only work inside a step; in a terminal they fail with `QAHYA_JOB_API is not set`.

To use values in the **same** step:

```sh
qahyacli env set X=1
echo "$(qahyacli env get X)"                                       # one value
eval "$(qahyacli env load abc.config --profile prod --export)"     # a whole config
```

`--export` prints `export NAME='value'` lines with every value single-quoted, so `eval` never executes anything inside a value. Don't combine it with `set -x`: the trace would print the values.

### Names

Names must match `[A-Za-z_][A-Za-z0-9_]*`. `PATH` and anything starting with `QAHYA_` are reserved for the runner.

## Files

### Job file (required)

The stand-in for Qahya's pipeline format: an ordered list of shell steps. Steps run with `sh -c` in the job file's directory. Unknown keys are errors; unnamed steps become `step-1`, `step-2`, …

```yaml
steps:
  - name: setup
    run: qahyacli env load abc.config --profile staging
  - name: deploy
    run: |
      echo "deploying to $DB_HOST"
```

### Config file (optional, for `env load`)

Holds variables and **references** to secrets, never secret values.

```yaml
version: 1

providers:
  local:
    type: file
    path: ./secrets.yaml          # relative to this file

defaults:                         # always loaded
  env:
    LOG_LEVEL: info

profiles:                         # --profile NAME, merged on top of defaults
  staging:
    env:
      DB_HOST: db.staging.internal
      VERSION: 1.10               # values are kept as written: "1.10", not 1.1
      DB_PASS:
        secret: local:db/staging#password
```

- An entry is either a plain value or `{ secret: provider:path#field }`.
- `FOO:` with no value is an error; write `FOO: ""` for an empty string.
- A key in the profile replaces the same key from `defaults`.
- Repeated loads merge into the store; variables from an earlier load stay unless overwritten.

### Secrets file (for the `file` provider)

Plain nested YAML. `local:db/staging#password` walks `db` → `staging` → `password`:

```yaml
db:
  staging:
    password: "st4g1ng-pass"
```

The `file` provider is a research stand-in for a real secret store; keep the real file out of git (the examples copy `secrets.example.yaml` into place).

## Job API

Plain HTTP over the socket, so it can be debugged with curl from inside a step:

```sh
curl --unix-socket "$QAHYA_JOB_API" http://qahya/env
```

| Request | Purpose |
|---|---|
| `GET /env` | list entries (secret values omitted) |
| `GET /env/{name}` | one entry, secret resolved |
| `PUT /env/{name}` `{"value": "..."}` | set a literal |
| `DELETE /env/{name}` | unset |
| `POST /load` `{"path", "profile", "export"}` | load a config; `path` must be absolute |

Errors come back as `{"error": "..."}` with a 4xx/5xx status. The socket lives in a `0700` temp dir and is removed when the job ends, including on Ctrl-C / SIGTERM.

## Layout

```
cmd/qahyacli/        main.go (dispatch), run.go (host role), env.go (client role)
internal/state/      Store, Entry, Resolver; name rules
internal/provider/   Provider interface; file provider
internal/config/     abc.config parsing, profiles, secret references
internal/jobapi/     protocol types, socket server, client
internal/runner/     job file; spawning steps
internal/shell/      quoting for --export
examples/            demo jobs, configs, broken inputs, run_tests.sh
```

Dependencies point one way: `cmd` → `runner`, `jobapi` → `config` → `state`, `provider`.

