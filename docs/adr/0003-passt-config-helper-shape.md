# ADR 0003: Discuss a narrow PasstConfig helper shape

- Status: Accepted; implemented in current codebase
- Date: 2026-06-22

> Implementation note: this ADR was written before the refactor landed. Phrases such as "future" or "if implemented" in the original discussion describe the decision context; current code uses `internal/model.PasstConfig` while keeping passt lifecycle policy in `internal/hvm`.

## Context

ADR-0001 made `passt` the default managed network backend and deliberately limited its similarity with `virtiofsd` to manager-owned helper lifecycle. At decision time, passt runtime fields still lived directly on `internal/hvm.Hypervisor`, while `virtiofsd` already had a model-level `VirtiofsConfig` value for per-share helper arguments and runtime paths.

This ADR records the decision to introduce a narrow `PasstConfig` value object that mirrors the useful parts of `VirtiofsConfig` without copying the `virtiofsd` reconciler or widening the public manifest surface.

## Decision

Introduce a narrow, node-scoped `PasstConfig` value object as an internal helper-runtime config, not a user-facing manifest contract.

A suitable shape would include only stable runtime helper data and rendering:

- one `PasstConfig` per node, not a slice and not one per manifest entry;
- manager-owned runtime artifact paths such as `SocketPath` and `PidPath`;
- a command renderer for the first implementation's passt command shape:

  ```bash
  passt --vhost-user --socket <node-run>/passt.sock --foreground
  ```

- comments making clear that `PidPath` is a `cvmm`-owned bookkeeping file containing the direct child PID, not a `passt --pid` path.

`PasstConfig` must not absorb lifecycle policy. Startup ordering, socket readiness, service-user/runtime-directory validation, process identity checks, cleanup, and fatal post-`vm.create` exit handling remain `internal/hvm` orchestration responsibilities.

## Rationale

`VirtiofsConfig` is useful because it bundles per-share helper paths and command arguments derived from `directory[]`. A `PasstConfig` can provide similar locality for the single passt helper's socket/pid paths and command shape.

The analogy is intentionally narrow:

- `virtiofsd` is per-share fan-out; `passt` is a single node-scoped helper.
- `virtiofsd` is reconciled; `passt` must be ready before `vm.create` and post-create exit is fatal.
- `virtiofsd` command arguments include filesystem and socket-group behavior; `passt` must not inherit those capabilities or group-sharing assumptions.
- `passt` pid ownership stays with `cvmm`; `passt --pid` is still out of scope.

## Non-goals

This ADR does not decide or introduce:

- new manifest fields for passt;
- multiple manifest-managed NICs;
- port forwarding or published ports;
- daemonized passt or helper-owned pid files;
- a passt reconciler/restart loop;
- reuse of `virtiofsd` ambient capabilities or socket-group behavior.

## Implementation guidance

Use a small type such as `model.PasstConfig` or `hvm.PasstConfig` with a minimal API:

```go
type PasstConfig struct {
    SocketPath string
    PidPath    string
}

func (p PasstConfig) CommandArgs() []string
```

The exact package is a code-organization choice:

- Use `internal/model` only if the type remains a pure runtime-path/argument value like `VirtiofsConfig`.
- Use `internal/hvm` if the type needs helper process identity or lifecycle-adjacent behavior.

The current implementation uses `internal/model` because the type remains limited to runtime paths plus command rendering.

Do not add service uid/gid, runtime directory policy, process handles, cancellation functions, or readiness state to the value object; those belong to `Hypervisor.Start` orchestration or a dedicated helper lifecycle component.

## Validation

The implementation includes tests for:

- passt command args include `--vhost-user`, `--socket`, and `--foreground`;
- passt command args do not include `--pid`;
- `PidPath` remains manager-owned and is not passed to passt.

Existing startup-order, readiness, fatal-exit, and cleanup tests continue to cover lifecycle behavior outside `PasstConfig`.

The implementation was also kept as an internal refactor: no new manifest or CLI surface was introduced.

Documentation should continue to point to ADR-0001 for passt backend semantics and to this ADR only for internal helper config shape discussion.
