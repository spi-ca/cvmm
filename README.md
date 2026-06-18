# cvmm

`cvmm`는 Go 기반 cloud-hypervisor VM 관리자다. 노드 저장소(`node-root`)의 YAML manifest와 이미지 저장소(`image-root`)를 읽어 VM 설정을 만들고, `cloud-hypervisor`와 `virtiofsd` 프로세스를 기동·종료·조작한다.

> 참고: 이전 ScreenFS에서 복사된 legacy artifact archive는 현재 `cvmm` evidence가 아니어서 제거했다. 새 측정 결과는 `cvmm` 명령, manifest, 이미지, host 환경, 원시 출력과 함께 별도 evidence bundle로 추가한다.

## 핵심 개념

- **이미지 저장소**: 기본값 `/srv/vmm/images`
  - 각 이미지 디렉터리는 보통 `vmlinuz`, `initramfs.img`, `root.img`를 가진다.
- **노드 저장소**: 기본값 `/srv/vmm/nodes`
  - 각 노드 디렉터리는 `config.yaml` manifest와 런타임 `run/` 디렉터리를 가진다.
- **주요 프로세스**
  - `cloud-hypervisor`
  - `virtiofsd` (manifest의 `directory` 항목별 1개)

## CLI

```text
cvmm start NODE_NAME
cvmm shutdown NODE_NAME
cvmm console NODE_NAME
cvmm console-file PTY_ID
cvmm client ACTION NODE_NAME
```

대표 플래그:

- `--image-root`, `--node-root`
- `--manifest-filename`
- `--cloudhypervisor-path`, `--virtiofsd-path`
- `--runas user`
- `--console`

플래그는 Viper로 environment variable에도 바인딩된다. 예: `IMAGE_ROOT`, `NODE_ROOT`, `CLOUDHYPERVISOR_PATH`.

## 노드 manifest

`<node-root>/<node>/config.yaml`이 source of truth다.

```yaml
cpus: 2
mem: 4G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net_mac_addr: 2e:33:5f:11:1b:42
net_if_name: vmtap-01
cmdline:
  - console=hvc0
  - quiet
disk:
  - data.img
directory:
  - configuration
```

동작 요약:

- `image`는 `<image-root>/<image>`를 가리킨다.
- `disk`의 relative path는 노드 디렉터리 기준 writable disk로 붙는다.
- `directory`의 각 항목은 virtio-fs 공유 디렉터리와 별도 `virtiofsd` 프로세스로 매핑된다.
- `net_mac_addr`, `net_if_name`가 비어 있으면 런타임에서 생성된다.
- `initramfs.img`가 없으면 initramfs 없이 기동한다.

## 저장소 안내

- `main.go` - CLI/flag/Viper entrypoint
- `internal/entry` - 서브커맨드별 진입점
- `internal/hvm` - hypervisor/virtiofsd orchestration, API client
- `internal/model` - manifest, cloud-hypervisor API payload, CLI arg rendering
- `internal/util` - PID file, PTY, 사용자/프로세스 유틸리티
- `contrib/cvmm@.service` - systemd 예시 유닛
- `docs/` - maintainer 문서

## 검증

문서 변경 최소 확인:

```bash
go test ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
```

코드 변경 시 추가 확인:

```bash
gofmt -w .
go vet ./...
go test ./...
```
