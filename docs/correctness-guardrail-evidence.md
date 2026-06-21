# Correctness guardrail evidence

이 문서는 현재 `cvmm` 저장소의 문서/코드 correctness guardrail과 그 근거를 정리한다. 목적은 `cvmm`가 실제 구현과 다른 런타임 계약, legacy non-cvmm 근거, 재현되지 않은 benchmark claim을 현재 evidence로 섞지 않도록 막는 것이다.

## 구현된 guardrail

`docs_smoke_test.go`는 기존 문서 inventory와 markdown local link 검증에 더해 아래 correctness smoke를 수행한다.

- active markdown과 Mermaid source에서 과거 프로젝트 identity 문자열이 발견되면 실패한다. `docs/guidelines/**`는 외부 guideline 보관 영역이므로 제외한다.
- legacy non-cvmm artifact나 존재하지 않는 benchmark harness를 현재 `cvmm` evidence처럼 주장하는 문구를 차단한다. 부정문/비목표/제거 기록은 허용하고, 긍정 claim을 실패시킨다.
- `.pi/extensions/guardrails.json`과 `.pi/extensions/guardrails.v0.json`이 JSON으로 파싱되고, `$schema`, `pathAccess`, `pathAccess.allowedPaths` key를 유지하는지 확인한다.
- current guardrail 파일인 `.pi/extensions/guardrails.json`에는 `version`이 있어야 하며, project-local 예외 경로는 object-shaped `directory` entries인 `/srv/vmm`, `/dev`로 고정한다.
- legacy 비교 파일인 `.pi/extensions/guardrails.v0.json`은 `allowedPaths`가 빈 목록인지 확인한다.

이 guardrail은 문서 drift를 조기에 잡는 smoke test다. 실제 VM 부팅, `/dev/kvm`, tap, systemd, 설치된 binary가 필요한 privileged integration evidence를 대체하지 않는다.

## 코드/문서 계약 근거

| 계약 | 현재 근거 |
| --- | --- |
| `start`는 `cloud-hypervisor` API socket으로 VMM을 띄운 뒤 readiness를 기다리고 VM create/boot를 수행한다. | `internal/hvm/hypervisor.go`의 `Start`, `Ping`, `cloudHypervisorProcessIdentity` |
| cloud-hypervisor API는 Unix socket 기반 REST API로 `/vmm.ping`, `/vm.create`, `/vm.boot`, `/vm.info` 등을 제공한다. | Cloud Hypervisor 공식 `docs/api.md` |
| manifest `directory[]`는 cloud-hypervisor fs device의 `tag/socket`과 host-side `virtiofsd` config로 변환된다. | `internal/model/config.go`의 `FsConfig`, `VirtiofsConfig` |
| virtio-fs는 host 공유 디렉터리를 guest에 제공하며, cloud-hypervisor에서는 `--fs tag=...,socket=...`와 shared memory가 필요하다. | Cloud Hypervisor 공식 `docs/fs.md` |
| `virtiofsd`는 `--socket-path`와 `--shared-dir`를 받는 vhost-user daemon이며, `--socket-group`, `--announce-submounts` 같은 옵션을 제공한다. | virtio-fs upstream `virtiofsd` README |
| share basename 중복은 guest tag/socket/pid 충돌을 만들 수 있어 load 단계에서 거부한다. | `internal/model/config.go`의 `ValidateDirectoryBasenames`, `internal/hvm/load.go` |
| `.pi` guardrail 설정은 현재 project-local 예외 경로를 `/srv/vmm`, `/dev` directory entries로 제한하고, legacy v0 비교 파일은 빈 예외 목록을 유지한다. | `.pi/extensions/guardrails.json`, `.pi/extensions/guardrails.v0.json`, `docs_smoke_test.go` |

## 권위 있는 외부 자료

초기 문서 작성 시점에는 현재 host에 `cloud-hypervisor`와 `virtiofsd` binary가 없어 재현 가능한 privileged VM integration을 수행하지 못했다. 이후 `/home/spi-ca/Codebase/scripts/cloud-hypervisor`와 `contrib/minimal` fixture로 제한적인 passt-backed minimal VM 기동 evidence를 추가했다. virtio-fs, production-style root, systemd까지 포함한 전체 런타임 계약의 외부 근거는 여전히 아래 upstream 문서를 기준으로 삼는다.

- Cloud Hypervisor API documentation: <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md>
  - REST API가 `--api-socket path=...` local UNIX socket으로 노출된다.
  - `/vmm.ping`, `/vm.create`, `/vm.boot`, `/vm.info`, `/vm.counters` 등의 endpoint와 request/response 계약을 설명한다.
- Cloud Hypervisor virtio-fs documentation: <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/fs.md>
  - `virtiofsd --socket-path=... --shared-dir=...` 실행 후 `cloud-hypervisor --fs tag=...,socket=...`로 guest에 공유 디렉터리를 연결하는 흐름을 설명한다.
  - `--memory shared=on` 필요성을 명시한다.
- virtio-fs upstream `virtiofsd` README: <https://gitlab.com/virtio-fs/virtiofsd>
  - `virtiofsd [FLAGS] [OPTIONS] --fd <fd>|--socket-path <socket-path> --shared-dir <shared-dir>` 사용법을 설명한다.
  - `--announce-submounts`, `--socket-group`, `--socket-path`, `--shared-dir`, sandbox/capability 관련 옵션을 설명한다.

## 로컬 evidence

2026-06-20 현재 세션에서 확인한 로컬 evidence:

```text
cloud-hypervisor: <not found in PATH>
virtiofsd: <not found in PATH>
/dev/kvm: present
/dev/net/tun: present
```

2026-06-22 추가 로컬 evidence:

```text
cloud-hypervisor: /home/spi-ca/Codebase/scripts/cloud-hypervisor, v52.0
passt: /usr/bin/passt, 2026_06_11.a9c61ff
minimal fixture: contrib/minimal/vmlinuz, contrib/minimal/root.img (file evidence and hashes in contrib/minimal/README.md)
run root: /tmp/cvmm-minimal-e2e.hj6CGu
manifest summary: cpus=2, mem=268435456, image=minimal, net.backend=passt, fixture-provided init command from contrib/minimal/run.sh
observed result: client vm-info state=Running
observed memory payload: {"size":268435456,"shared":true,"thp":false}; mergeable omitted
```

이 evidence는 minimal passt-backed VM이 cloud-hypervisor v52.0에서 `mergeable=on,shared=on` validation error 없이 `Running`까지 도달했음을 보여준다. 표준 `cvmm shutdown` 완료, virtiofsd fan-out, systemd start/stop timing은 이 evidence에 포함하지 않는다.

검증 명령 evidence:

```bash
gofmt -w .
go test ./...
go vet ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
python3 - <<'PY'
import json
for p in ['.pi/extensions/guardrails.json','.pi/extensions/guardrails.v0.json','.pi/settings.json']:
    json.load(open(p))
print('json ok')
PY
git diff --check
```

관찰 결과:

- `go test ./...` 통과
- `go vet ./...` 통과
- docs inventory listing 통과; `docs/correctness-guardrail-evidence.md`가 listing에 포함됨
- `.pi/extensions/guardrails.json`, `.pi/extensions/guardrails.v0.json`, `.pi/settings.json` JSON parse 통과
- `git diff --check` 통과

## 남는 integration evidence

아래 항목은 설치된 binary와 runtime fixture가 있어야 하므로 별도 privileged host evidence로 남긴다.

- 실제 `virtiofsd` version 기록과 virtio-fs share fixture provenance
- `/srv/vmm/images`, `/srv/vmm/nodes` production-style fixture와 manifest provenance
- `cvmm start NODE` end-to-end 실행 로그의 durable artifact 보관
- `/vmm.ping` readiness timestamp와 `vm.create`/`vm.boot` timing 결과
- running VM 기준 `client vm-counters`, `console NODE`, 정상 `shutdown NODE` evidence
- systemd unit verification은 host-installed `/usr/bin/cvmm` 또는 동등한 override path가 있을 때 재시도
