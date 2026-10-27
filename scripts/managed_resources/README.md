# Managed comparison resource-stage integration

This test-only entry supplies the resource envelope missing from the PR7 attempt.
It does not implement or admit a managed comparison, launch a Go loader, or
qualify the filesystem, socket/network, exec, descriptor or cache boundaries.
Production comparison and classification remain unchanged.

The Python runner and preflight/probe tests are vendored from the RelayWeft
resource-launcher implementation. They use only Python's standard library and
systemd user services; no path to a RelayWeft checkout is required. `source-sha256.json`
records the imported files' bytes. Future changes require review and rerunning
both mocked and host tests; the manifest is provenance, not runtime attestation.

## Preconditions

Linux with systemd user manager and cgroup v2 memory/pids delegation. The tested
WSL host had systemd 255.4 and delegated cpu/memory/pids. No sudo, package install,
persistent service or delegation changes are made by these scripts.

Run from the Goquent repository in a normal host terminal, not an unapproved
sandbox escape. Existing tool approvals still apply to host-side commands.

```sh
python3 scripts/managed_resources/diagnose-wsl-resources.py
python3 scripts/managed_resources/verify-wsl-resources.py --run
python3 scripts/managed_resources/test-host-integration.py --run
```

The last command creates three temporary services, sequentially: normal exit
with a child process, command exit 7, and deadline termination. It stops on the
first unconfirmed case. It never executes project code or allocates excessive
memory/processes. Unit removal must be observed, not inferred from an error.

## Supervisor integration contract

A future reviewed supervisor may be invoked as follows:

```text
python3 scripts/managed_resources/run-resource-limited.py \
  --cwd /absolute/owned-fixture --timeout-seconds EXPLICIT_TOTAL_DEADLINE \
  -- /absolute/reviewed-supervisor ARGUMENTS
```

This is an interface example, not an existing supervisor or a command to run now.
The runner checks actual memory.max=2147483648, pids.max=64 and its dedicated
unit membership immediately before exec. A mismatch prevents exec; there is no
unrestricted fallback. The executable path must be absolute, with no implicit
shell, PATH lookup or systemd environment expansion.

The cap includes the supervisor and its ordinary descendants. 64 tasks includes
threads. Swap remains unchanged; this is not a combined RAM-plus-swap limit.
The total deadline does not replace the design's individual preparation/load
budgets. The supervisor must explicitly provision and verify its environment,
offline modules, selection and cache lifecycle. A user service does not inherit
all environment variables from the invoking shell.

Command stdout/stderr stream to the caller; nothing is automatically uploaded.
Resource events on stderr are prefixed `RELAYWEFT_RESOURCE`. Exit zero requires
successful unit execution and confirmed cleanup. Cleanup uncertainty returns
125; timeout or interrupted client paths return nonzero. Unit failure alone
must not be labeled approval refusal or a successful comparison.

This is not protection against a malicious command that accesses the user bus
or launches work outside its domain. The remaining reviewed isolation boundary
must prevent escape before any managed admission. No eligible pair, narrower
witness, full/view parity or release acceptance is established here.

## Validation without services

```sh
python3 scripts/managed_resources/test-wsl-resources.py
python3 scripts/managed_resources/test-wsl-resource-verification.py
python3 scripts/managed_resources/test-resource-limited-runner.py
```

The host test is opt-in; merely importing or running it without `--run` creates
no units. Report its actual result separately from mocked unit tests.

## User-reported host acceptance (2026-10-28 JST)

The user ran the opt-in host integration test from Windows into Ubuntu. All
three cases passed: normal exit 0, command failure exit 7, deadline exit 1.
Every case reported limits_verified=true and cleanup_verified=true. No stress
test or managed comparison was performed. This does not establish the remaining
isolation or loader contracts.

Current approved scope is branch separation and resource-stage organization
only. Supervisor, isolation and real-loader implementation are not part of this
branch.
