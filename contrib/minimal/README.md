# Minimal boot fixture

This directory contains a tiny local boot fixture for cvmm/cloud-hypervisor smoke testing.

## Contents

- `vmlinuz`: Linux x86 bzImage kernel.
- `root.img`: EROFS root filesystem image.
- `run.sh`: direct cloud-hypervisor reference command for the fixture.

The fixture is intentionally minimal and boots with the image-provided init command from `run.sh`. It is not a general-purpose guest image and is not a benchmark artifact.

## Current file evidence

Captured on 2026-06-22 in this repository worktree:

```text
vmlinuz:  Linux kernel x86 boot executable, bzImage, version 6.19.6-1-spica-vm
root.img: EROFS filesystem
```

SHA-256:

```text
9f6e06439f0f3dd40263e4cf0d945b877af31e3e6675d7ef675f5a66c539662a  vmlinuz
4d1879a6824934ff7d6776908e2891e39808350eeff17cd919fc33a290560cc7  root.img
8feca865fef4e146652776b6d1749bc7bc8a46682c0b184cb1a7556444e56bfa  run.sh
```

## Usage

Use `run.sh` as the direct cloud-hypervisor reference command. cvmm tests should copy `vmlinuz` and `root.img` into a temporary image root and create a temporary node manifest instead of writing runtime state into this directory.

The fixture was used to confirm a passt-backed cvmm node reached `state: Running` with cloud-hypervisor v52.0 after the default shared-memory payload stopped sending `mergeable=on`.

## Limitations

- No build recipe is currently stored here; treat the images as checked-in test fixtures with the hashes above.
- Do not claim benchmark results from this fixture.
- This fixture does not cover virtio-fs fan-out, systemd unit behavior, or production image provenance.
