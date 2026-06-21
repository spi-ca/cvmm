# ADR-001 network migration note

ADR-001 changed the manifest-managed network default from TAP-style top-level fields to a nested `net` tree with `passt` as the default backend.

## What changed

New/default form:

```yaml
net:
  backend: passt
  mac_addr: 2e:33:5f:11:1b:42
```

TAP compatibility form:

```yaml
net:
  backend: tap
  mac_addr: 2e:33:5f:11:1b:42
  if_name: vmtap-01
```

Important rules:

- omitted `net.backend` means `passt`;
- `net.if_name` is TAP-only;
- a manifest with `net.if_name` or legacy `net_if_name` must set `net.backend: tap`;
- legacy `net_mac_addr` and `net_if_name` are compatibility inputs, but new manifests should use `net.mac_addr` and `net.if_name`;
- `--runas` still applies to the `cloud-hypervisor` child only, not to managed helpers.

## Manifest migration examples

Before, TAP implicit:

```yaml
image: debian-bookworm
net_mac_addr: 2e:33:5f:11:1b:42
net_if_name: vmtap-01
```

After, keep TAP explicitly:

```yaml
image: debian-bookworm
net:
  backend: tap
  mac_addr: 2e:33:5f:11:1b:42
  if_name: vmtap-01
```

After, move to default `passt`:

```yaml
image: debian-bookworm
net:
  mac_addr: 2e:33:5f:11:1b:42
```

When moving to `passt`, remove TAP-only `if_name` unless you intentionally keep `net.backend: tap`.

## Operational checklist for `passt`

Before enabling or relying on default `passt` for a node:

1. Ensure `passt` exists, or pass `--passt-path` / `PASST_PATH`.
2. Run `cvmm start` as a dedicated non-root service user/group.
3. Do not rely on root manager + `--runas` to lower helper privileges.
4. Ensure `<node-root>/<node>/run/` is a real directory, not a symlink.
5. Ensure the `run/` directory is owned by the service uid and is not more permissive than `0700`.
6. If TAP behavior is required, set `net.backend: tap` and keep any `if_name` there.

## Validation hints

For a migrated node, collect evidence for:

- effective backend (`passt` default or explicit `tap`);
- `run/passt.sock` readiness and `run/passt.pid` direct-child PID ownership for `passt`;
- cleanup of `passt.sock`/`passt.pid` after shutdown or startup failure;
- `CAP_NET_ADMIN` only on the TAP path;
- `go test ./...` and `go vet ./...` for code changes.
