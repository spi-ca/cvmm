# cvmm 요구사항

## 목적

`cvmm`는 노드별 YAML manifest를 읽어 `cloud-hypervisor` VM을 만들고, 필요한 `passt`/`virtiofsd` 보조 프로세스를 함께 관리한다. 이미지 저장소와 노드 저장소를 분리해 운영자가 같은 이미지 세트를 여러 노드에 재사용할 수 있게 하는 것이 기본 모델이다.

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
- `net.backend` (optional, default `passt`)
- `net.mac_addr` (optional)
- `net.if_name` (optional, TAP only)
- legacy `net_mac_addr` (compatibility input)
- legacy `net_if_name` (compatibility input)
- `cmdline` (optional)
- `disk` (optional)
- `directory` (optional)

의미:

- `image`는 이미지 저장소 하위 디렉터리 이름이다.
- `disk`는 writable block device 목록이다. relative path는 노드 디렉터리 기준으로 해석한다.
- `directory`는 virtio-fs export 목록이다. 각 항목마다 `virtiofsd` 인스턴스를 별도로 만든다.
- `directory` 항목의 basename은 share별 guest tag, socket, pid suffix로 쓰이므로 중복되면 manifest load가 실패해야 한다.
- `net.backend`를 비우면 `passt`가 기본값이다.
- `net.if_name`는 TAP 전용이므로 `net.backend: tap` 없이 값이 있으면 actionable error로 거부해야 한다.
- legacy top-level `net_mac_addr`, `net_if_name`는 nested `net`으로 병합되지만, TAP 유지에는 `net.backend: tap`을 명시해야 한다.
- `net.mac_addr`가 없으면 런타임에서 생성한다.
- TAP backend에서 `net.if_name`가 없으면 런타임 기본값을 생성한다.

지원 network 예:

```yaml
net:
  backend: passt
  mac_addr: 2e:33:5f:11:1b:42
```

```yaml
net:
  backend: tap
  mac_addr: 2e:33:5f:11:1b:42
  if_name: vmtap-01
```

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
- `--passt-path`와 대응 environment binding으로 `passt` 바이너리 경로를 지정할 수 있어야 한다.

## 런타임 요구사항

- `cloud-hypervisor` 바이너리 경로를 지정할 수 있어야 한다.
- `virtiofsd` 바이너리 경로를 지정할 수 있어야 한다.
- `passt` 바이너리 경로를 지정할 수 있어야 한다.
- pid file, API socket, `passt` socket/pid, virtiofs socket, virtiofs helper pid file은 노드 `run/` 디렉터리 아래에 놓는다.
- `--runas`가 주어지면 `cloud-hypervisor` child process credential 전환을 적용할 수 있어야 한다.
- `virtiofsd` helper 권한은 서비스 계정, capability, `--socket-group`, shared-directory, absolute-path directory, and submount exposure 모델에 따라 관리되어야 한다.
- `virtiofsd` helper pid file은 share별 경로로 기록되고 helper 종료 후 정리되어야 한다.
- `passt`는 node-scoped helper 하나만 두고 `vm.create` 전에 시작되며 `passt.sock` readiness를 기다려야 한다.
- `passt.pid`는 helper 자체가 아니라 `cvmm`가 direct child PID로 기록/정리해야 한다.
- `vm.create` 이후 `passt` 비정상 종료는 fatal lifecycle error로 처리되어야 한다.
- `CAP_NET_ADMIN`은 TAP backend에만 적용되어야 한다.
- `passt` backend의 `cloud-hypervisor` child는 networking 때문에 ambient capability를 추가로 받지 않아야 한다.
- 모든 backend와 non-start socket/pid 접근 경로는 안전한 `run/` directory owner/mode/symlink 검증을 요구해야 한다.
- `passt` backend는 추가로 dedicated non-root service user/group을 요구해야 한다.
- root manager + `--runas`만으로 helper를 낮추는 현재 배포 패턴은 `passt` backend 지원 요구사항으로 간주하지 않는다.
- 시작 시 중복 pid file과 이미 실행 중인 프로세스를 감지해야 한다.

## 비목표

- 이미지 빌드 시스템 제공
- 스케줄러/클러스터 오케스트레이터 제공
- guest OS provisioning 자동화
- 현재 `cvmm` 범위와 무관한 레거시 계약을 요구사항으로 계승
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
