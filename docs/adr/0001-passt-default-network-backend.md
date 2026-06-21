# ADR 0001: Use passt as the default managed network backend

- Status: Accepted; implemented in current codebase
- Date: 2026-06-21

> Implementation note: this ADR was written before the code change. Phrases such as "currently" in the original context describe the repository at decision time; current behavior is documented in `README.md`, `docs/design.md`, `docs/architecture.md`, and `docs/operations.md`.

## Context

At decision time, `cvmm` implemented a single manifest-managed TAP network device for `start`:

- the current manifest exposes top-level `net_mac_addr` and `net_if_name`;
- `hvm.Load` generates a `vmtap-*` interface name when one is not supplied;
- `model.Config.NetConfig` renders a single cloud-hypervisor `tap=...` network entry;
- `Hypervisor.Start` gives the `cloud-hypervisor` child `CAP_NET_ADMIN` unconditionally.

This ADR recorded the intended direction for the implementation that is now present in the current codebase.

The local evaluation found that `passt` can provide a vhost-user socket that cloud-hypervisor can consume with a network payload using `vhost_user`, `vhost_socket`, and `vhost_mode: "Client"`.

For `passt`-specific behavior, the upstream/local `passt` man page, documentation, and source are authoritative references for this project, analogous to the way cvmm treats cloud-hypervisor and virtiofsd documentation for their command/API behavior.

## Decision

### Default backend and manifest selector

1. `passt` will become the default manifest-managed network backend.
2. The user-facing network configuration will move under a nested manifest tree named `net`. The backend selector is `net.backend`, with accepted values `passt` and `tap`:

   ```yaml
   net:
     backend: passt
     mac_addr: 2e:33:5f:11:1b:42
   ```

3. After this ADR is implemented, an omitted `net.backend` means `passt`; `net.backend: tap` is the explicit compatibility opt-in for the current TAP behavior. A CLI override can be considered later, but it is not part of this decision.
4. The current top-level `net_mac_addr` manifest field will become nested `net.mac_addr` and applies to both backends for the first implementation.
5. The current top-level `net_if_name` manifest field will become nested `net.if_name` and is TAP-only:

   ```yaml
   net:
     backend: tap
     mac_addr: 2e:33:5f:11:1b:42
     if_name: vmtap-01
   ```

   If a manifest selects `passt` or omits `net.backend` after this ADR is implemented, a non-empty `net.if_name` must be rejected with an error that tells the operator to set `net.backend: tap` to keep TAP behavior. This is an intentional migration gate for legacy TAP manifests rather than implicit TAP fallback.

### Helper management parity with virtiofsd

`passt` will be managed like `virtiofsd` only in the manager-owned helper sense:

- `cvmm start` starts the helper;
- `cvmm` monitors the helper process and signals it during shutdown;
- unexpected `passt` exit after VM creation is fatal for the managed VM lifecycle;
- runtime artifacts live below `<node-root>/<node>/run/`;
- `cvmm` owns pid-file creation and cleanup, as it does for `virtiofsd` helpers;
- `passt.pid` records the direct child host PID observed by `cvmm`, not a PID emitted by `passt` itself;
- `--runas` remains a cloud-hypervisor child setting and does not directly lower helper privileges.

This parity does **not** mean copying the current `virtiofsd` implementation wholesale. `passt` must not reuse the share fan-out reconciler, the filesystem-oriented `virtiofsd` ambient capability set, or the current `runas` primary-group-to-helper-socket-group behavior.

### Passt-specific exceptions

For the first implementation:

1. `passt` is a single node-scoped helper, not one helper per manifest entry.
2. `passt` must be launched and reach socket readiness before `vm.create`, because cloud-hypervisor needs the vhost-user socket when attaching the manifest-managed NIC.
3. TCP/UDP port forwarding, published ports, and host listener exposure are out of scope.
4. The initial manifest-managed network remains a single NIC. This limits only the `start` path's default/manifest-created NIC; existing explicit client actions such as `vm-add-net` are outside this decision.
5. TAP remains a compatibility backend, but it will no longer be the default once this ADR is implemented.

### Binary discovery

`passt` binary discovery will match the existing helper pattern: add a `--passt-path` flag and matching environment binding, with a default absolute path of `/usr/bin/passt`, resolved through the same binary lookup path used for `cloud-hypervisor` and `virtiofsd`.

## Target operational semantics

The following sections describe the implemented target state for this ADR.

### Runtime artifacts and command shape

A passt-backed node should use runtime files similar to:

```text
<node-root>/<node>/run/passt.sock
<node-root>/<node>/run/passt.pid
```

The helper command is expected to be shaped like, using the binary resolved from `--passt-path`:

```bash
passt \
  --vhost-user \
  --socket <node-run>/passt.sock \
  --foreground
```

`--foreground` is mandatory for the first implementation. It keeps `passt` as a direct child of the `cvmm start` manager, so `cmd.Process.Pid`, parent-death signaling, process waiting, and fatal-exit handling refer to the long-lived helper process. This deliberately does not use passt's default daemonization behavior.

The pid file is intentionally omitted from the helper command shape. To match the current `virtiofsd` ownership model, `cvmm` should write and remove `<node-run>/passt.pid` around the spawned helper process instead of delegating pid-file ownership to `passt --pid`.

`passt --pid` is not part of the first implementation. Upstream `passt` documents `--pid` as writing its own PID after initialization and before background forking if backgrounding is configured; upstream source also supports writing that PID file in foreground mode. The first cvmm implementation still keeps PID-file creation and cleanup in `cvmm` so the ownership model matches existing helpers and uses the direct child host PID observed by the manager. A future switch to helper-owned `passt --pid` or daemonized passt would require a separate ADR covering cgroup/systemd tracking, parent-death behavior, host PID visibility, stale cleanup, and process-identity verification.

`passt.pid` is a lifecycle artifact, not a readiness signal. `vm.create` must still wait for `passt.sock` readiness.

The cloud-hypervisor VM network payload should be vhost-user based rather than TAP based. In API/JSON form this means fields like:

```yaml
vhost_user: true
vhost_socket: <node-run>/passt.sock
vhost_mode: Client
```

When rendered as cloud-hypervisor CLI fragments, the socket field is expected to use cloud-hypervisor's `socket=...` argument form, not the API field name `vhost_socket`.

### Startup ordering and cleanup

`passt` must be started before `vm.create`. The implementation must explicitly wait for socket readiness and must clean up `passt` on all failure paths, including:

- `passt` readiness failure;
- cloud-hypervisor readiness failure;
- `vm.create` failure;
- context cancellation before boot;
- normal shutdown;
- abrupt helper exit.

After `vm.create`, an unexpected `passt` exit is treated as a fatal lifecycle error for the manifest-managed VM. The first implementation should not attempt transparent `passt` restart/reconnect, because the single vhost-user NIC may not reconnect safely after the helper process disappears. Instead, `cvmm` should stop ancillary processes, request or force VM shutdown according to the existing shutdown path, clean up `passt` runtime files, and return an error from `start`.

A simple single-helper launcher with explicit cleanup is preferred over copying the share fan-out `virtiofsd` reconciler directly.

### Capability, credential, and socket access model

- `CAP_NET_ADMIN` should be needed only for the TAP backend. Passt-backed cloud-hypervisor execution should not receive `CAP_NET_ADMIN` merely because networking is enabled.
- `passt` must not inherit the current `virtiofsd` ambient capability set. The first version should give `passt` no ambient capabilities; because port forwarding is out of scope, `CAP_NET_BIND_SERVICE` is also out of scope.
- The first version supports passt backend only when the manager runs under a dedicated non-root service user/group. A root manager that relies on `--runas` only to lower cloud-hypervisor remains unsupported for passt until a later design explicitly covers it. The current example `contrib/cvmm@.service` pattern with `--runas` but no enforced `User=`/`Group=` is therefore not sufficient for passt deployment as-is.
- Because `passt` can drop privileges internally, the implementation must pass or otherwise enforce the intended service uid/gid rather than relying on passt's default credential behavior.
- If `--runas` makes cloud-hypervisor run as a different uid/gid from the service user, the implementation must provide a dedicated private group, ownership handoff, or equivalent narrow socket access control for `passt.sock`; otherwise the `--runas` + `passt` combination must remain unsupported.
- `<node-root>/<node>/run` must be owned by the service user. For passt mode, `start` should fail closed if the directory is more permissive than `0700`, or more permissive than `0750` when using a dedicated private group for socket sharing. Do not reuse any helper that silently creates the passt runtime directory as `0755`.
- Passt sockets should not be exposed to shared human login groups. If group access is needed, use a dedicated VM/private group that grants access only to the service user and the cloud-hypervisor child identity.

## Rollout and migration

- Documentation that describes current behavior must say that the default manifest-managed backend is `passt` and TAP requires explicit `net.backend: tap`.
- Implementing this ADR intentionally changes the default backend for manifests that omit `net.backend`.
- Existing manifests that rely on TAP-specific `net_if_name` must migrate it to `net.if_name` and add `net.backend: tap` during migration.
- Existing deployments that run the manager as root and rely on `--runas` only to lower cloud-hypervisor must either keep `net.backend: tap` or move to a dedicated non-root service user/group with safe `passt.sock` access control before using the default passt backend.
- The implementation must fail with an actionable error for `net.if_name` without `net.backend: tap`; it must not silently infer TAP from the TAP-only field.
- Multi-NIC manifest support and port forwarding require separate design work.

## Consequences

- Implementing this ADR requires changes to manifest loading, network defaulting, VM config generation, `hvm.Load`, `Hypervisor.Start`, capability selection, tests, and user documentation.
- Tests must distinguish the default passt path from the compatibility TAP path.
- Operational docs must explain that `--runas` does not lower helper privileges under this decision.
- The `virtiofsd` comparison is deliberately narrow: lifecycle ownership is shared; process count, startup order, readiness checks, credentials, and capabilities differ.

## Validation expected when implemented

The implementation should include at least:

- manifest/model tests for default `passt`, explicit `tap`, nested `net.mac_addr`, TAP-only `net.if_name` rejection without `net.backend: tap`, and single manifest-managed NIC behavior;
- VM config tests asserting `vhost_user: true`, `vhost_socket`, and `vhost_mode: "Client"` for passt;
- start-path tests that `passt` is started before `vm.create`, waits for socket readiness, treats post-create `passt` exit as fatal, and is cleaned up on every failure path;
- pid ownership tests showing `cvmm` writes and removes `passt.pid`;
- capability tests showing `CAP_NET_ADMIN` is only attached for TAP and no `virtiofsd` filesystem capabilities are attached to `passt`;
- runtime-directory permission tests that reject unsafe owner/mode combinations for passt mode;
- command-shape tests showing `--foreground` is present and `--pid` is absent for the first passt implementation;
- docs-only checks plus `go test ./...` and `go vet ./...` for code changes.
