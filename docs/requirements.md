# cvmm 요구사항

## 목적

`cvmm`는 노드별 YAML manifest를 읽어 `cloud-hypervisor` VM을 만들고, 필요한 `virtiofsd` 보조 프로세스를 함께 관리한다. 이미지 저장소와 노드 저장소를 분리해 운영자가 같은 이미지 세트를 여러 노드에 재사용할 수 있게 하는 것이 기본 모델이다.

## 입력 계약

### 이미지 저장소

기본 경로는 `/srv/vmm/images`다.

각 이미지 디렉터리는 보통 아래 파일명을 사용한다.

- `vmlinuz`
- `initramfs.img`
- `root.img`

`initramfs.img`는 없을 수 있으며, 없으면 initramfs 없이 기동한다.

### 노드 저장소

기본 경로는 `/srv/vmm/nodes`다.

각 노드 디렉터리는 최소한 아래를 가진다.

- `config.yaml`
- optional writable disk/image files referenced by manifest
- `run/` runtime directory for pid/socket files

### manifest

현재 코드가 직접 읽는 핵심 필드:

- `cpus`
- `mem`
- `uuid`
- `image`
- `net_mac_addr` (optional)
- `net_if_name` (optional)
- `cmdline` (optional)
- `disk` (optional)
- `directory` (optional)

의미:

- `image`는 이미지 저장소 하위 디렉터리 이름이다.
- `disk`는 writable block device 목록이다. relative path는 노드 디렉터리 기준으로 해석한다.
- `directory`는 virtio-fs export 목록이다. 각 항목마다 `virtiofsd` 인스턴스를 별도로 만든다.
- `net_mac_addr`, `net_if_name`가 없으면 런타임 기본값을 생성한다.

## CLI 요구사항

지원 서브커맨드:

- `start NODE_NAME`
- `shutdown NODE_NAME`
- `console NODE_NAME`
- `console-file PTY_ID`
- `client ACTION NODE_NAME`

지원 요구:

- 플래그와 environment variable 둘 다 동작해야 한다.
- `client`는 cloud-hypervisor UNIX socket API를 호출해야 한다.
- request body가 필요한 `client` action은 YAML을 stdin으로 받아야 한다.

## 런타임 요구사항

- `cloud-hypervisor` 바이너리 경로를 지정할 수 있어야 한다.
- `virtiofsd` 바이너리 경로를 지정할 수 있어야 한다.
- pid file, API socket, virtiofs socket은 노드 `run/` 디렉터리 아래에 놓는다.
- `--runas`가 주어지면 `cloud-hypervisor` child process credential 전환을 적용할 수 있어야 한다.
- `virtiofsd` helper 권한은 서비스 계정, capability, `--socket-group`, shared-directory, absolute-path directory, and submount exposure 모델에 따라 관리되어야 한다.
- 시작 시 중복 pid file과 이미 실행 중인 프로세스를 감지해야 한다.

## 비목표

- 이미지 빌드 시스템 제공
- 스케줄러/클러스터 오케스트레이터 제공
- guest OS provisioning 자동화
- 레거시 ScreenFS/FUSE 계약을 현재 `cvmm` 요구사항으로 계승
- 현재 저장소에 없는 formal benchmark harness를 이미 있는 것처럼 문서화

## 검증 요구사항

문서나 코드 변경 후 기본 확인:

```bash
go test ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
```

코드 변경 시 추가 확인:

```bash
gofmt -w .
go vet ./...
```
