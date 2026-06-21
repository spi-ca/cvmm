# AGENTS.md

## Project

- Name: `cvmm`
- Module: `amuz.es/src/spi-ca/cvmm`
- Language: Go
- Role: manage `cloud-hypervisor` VMs and per-share `virtiofsd` helpers from YAML node manifests.

## Runtime model

- Image root default: `/srv/vmm/images`
- Node root default: `/srv/vmm/nodes`
- Node manifest default: `<node-root>/<node>/config.yaml`
- Runtime files default: `<node-root>/<node>/run/`

`start` loads a node manifest, resolves image artifacts, starts `cloud-hypervisor` and, when the effective backend is `passt`, a node-scoped `passt` helper, waits for readiness, creates the VM, boots it, and keeps auxiliary `virtiofsd` processes reconciled.

Current manifest-managed networking defaults to nested `net` configuration with `passt`. Explicit TAP compatibility remains available with `net.backend: tap`; `CAP_NET_ADMIN` is only required for TAP, and `passt` mode requires a dedicated non-root service user plus restricted `<node>/run/` ownership/mode checks. `--runas` remains a `cloud-hypervisor` child setting and cannot be used to deploy `passt` from a root manager.

## CLI surface

```text
cvmm start NODE_NAME
cvmm shutdown NODE_NAME
cvmm console NODE_NAME
cvmm console-file PTY_ID
cvmm client ACTION NODE_NAME
```

Some `client` actions read YAML request bodies from stdin.

## Editing guardrails

- Do not describe this repository with unrelated legacy project identities or stacks.
- Keep docs aligned with `main.go`, `internal/entry`, `internal/hvm`, and `internal/model`.
- Keep docs explicit about current default-`passt` behavior, explicit `net.backend: tap` compatibility, `passt` helper lifecycle/runtime artifacts, runtime permission checks, and `--runas` limitations.
- Do not reintroduce legacy non-cvmm artifact archives; add only explicit `cvmm` evidence with provenance.
- Do not modify `docs/guidelines/**` unless explicitly requested.

## Validation

```bash
go test ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
```

For code changes also run:

```bash
gofmt -w .
go vet ./...
```
