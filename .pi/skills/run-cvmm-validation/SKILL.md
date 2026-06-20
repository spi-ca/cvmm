---
name: run-cvmm-validation
description: Run and record cvmm validation or measurement evidence for startup, orchestration, manifest, or service changes.
---

# Repo Validation and Measurement Helper

This skill records validation and measurement evidence for the current cvmm repository.

## When to Use

- A change touches Go startup/shutdown paths, cloud-hypervisor or virtiofsd orchestration, manifest loading, or the systemd service wrapper.
- Review needs reproducible validation or timing evidence beyond a plain statement that the code "looks correct".
- You need a compact evidence bundle for current cvmm behavior, not an unrelated legacy benchmark run.

Do not use this skill for routine docs-only edits unless the task explicitly asks for measurement evidence.

## Procedure

1. Pick the validation surfaces that match the changed files.
   - Go code, CLI flags, command builders, or config structs: run `go test ./...`.
   - YAML/OpenAPI files: parse the touched files with a local YAML parser.
   - `contrib/cvmm@.service`: run `systemd-analyze verify contrib/cvmm@.service` when the tool is available.
2. If timing evidence is requested, wrap the exact repo-native command with a non-root timing tool such as `/usr/bin/time` and record the full command line.
3. Prefer focused measurements around cvmm behavior, for example start/shutdown flows, command construction, manifest loading, or relevant tests.
4. Keep raw command output as the source of truth and write summaries separately.
5. Record environment assumptions that affect the result: host tool availability, relevant binary paths, current worktree state, and any skipped checks.
6. Re-run the correctness checks after performance or timing experiments if the implementation changed during the investigation.

## Pitfalls

- Do not require root-only workflows or global system mutation.
- Do not assume `cloud-hypervisor`, `virtiofsd`, or `systemd-analyze` exist; report missing tools explicitly.
- Do not treat YAML parsing alone as semantic proof when Go config builders changed.
- Do not reuse unrelated legacy filesystem semantics, hidden-path rules, or benchmark claims in cvmm evidence.
- Do not claim timing evidence without the exact command line and environment notes.

## Verification

- Touched Go code passes `go test ./...`.
- Touched YAML/OpenAPI files parse successfully.
- The touched systemd unit verifies successfully, or the missing verifier is reported.
- Any recorded measurement includes command lines, environment notes, and the corresponding correctness checks.
