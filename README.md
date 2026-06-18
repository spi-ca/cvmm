# cvmm

`cvmm`는 YAML node manifest 하나를 기준으로 `cloud-hypervisor` VM과 share별 `virtiofsd` helper를 관리하는 Go CLI다. 이미지 저장소와 노드 저장소를 분리해, 같은 OS 이미지 세트를 여러 VM 노드에 재사용하면서 노드별 disk/share/network/console 설정을 manifest로 선언한다.

![cvmm purpose](docs/diagrams/project-purpose.svg)

## 언제 쓰나

`cvmm`는 아래 상황에 맞춘 작은 VM runtime manager다.

- host에 이미 준비된 `cloud-hypervisor`와 `virtiofsd`로 VM을 기동해야 할 때
- VM별 설정을 `<node-root>/<node>/config.yaml` 파일로 관리하고 싶을 때
- 공통 image repository의 `vmlinuz`, optional `initramfs.img`, `root.img`를 여러 node가 재사용해야 할 때
- node별 writable disk와 virtio-fs 공유 디렉터리를 manifest로 붙이고 싶을 때
- systemd나 자동화에서 `start`, `shutdown`, `client`, `console` 명령을 호출하고 싶을 때

비목표: 이미지 빌드, 클러스터 스케줄링, guest provisioning 자동화, 현재 저장소에 없는 benchmark harness 제공.

## 핵심 모델

- **image root**: 기본값 `/srv/vmm/images`
  - 각 image directory는 보통 `vmlinuz`, `initramfs.img`, `root.img`를 가진다.
  - `initramfs.img`는 없어도 된다.
- **node root**: 기본값 `/srv/vmm/nodes`
  - 각 node directory는 `config.yaml`, optional writable disk/share, `run/` runtime directory를 가진다.
- **runtime process**
  - `cloud-hypervisor`: VM lifecycle과 Unix socket API 제공
  - `virtiofsd`: manifest `directory[]` 항목마다 하나씩 실행

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

## Manifest 예시

`<node-root>/<node>/config.yaml`이 node source of truth다.

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
- `disk[]`의 relative path는 node directory 기준 writable disk로 붙는다.
- `directory[]`의 각 항목은 virtio-fs 공유 디렉터리와 별도 `virtiofsd` process로 매핑된다.
- `net_mac_addr`, `net_if_name`가 비어 있으면 런타임에서 생성된다.
- `initramfs.img`가 없거나 디렉터리면 initramfs 없이 기동한다.

Schema와 runtime mapping은 다음 다이어그램을 참고한다.

![cvmm manifest schema](docs/diagrams/manifest-schema.svg)

![cvmm manifest runtime mapping](docs/diagrams/manifest-runtime-mapping.svg)

## 기본 사용 예

VM 시작:

```bash
cvmm start NODE_NAME
```

경로와 binary를 명시해 시작:

```bash
cvmm \
  --image-root /srv/vmm/images \
  --node-root /srv/vmm/nodes \
  --cloudhypervisor-path /usr/bin/cloud-hypervisor \
  --virtiofsd-path /usr/lib/virtiofsd \
  --runas hvm \
  start NODE_NAME
```

VM 종료:

```bash
cvmm shutdown NODE_NAME
```

cloud-hypervisor API 조회:

```bash
cvmm client vm-info NODE_NAME
```

request body가 필요한 client action은 YAML을 stdin으로 받는다.

```bash
cat request.yaml | cvmm client vm-resize NODE_NAME
```

Console attach:

```bash
cvmm console NODE_NAME
cvmm console-file PTY_ID
```

`start` lifecycle와 `client ACTION` 처리 흐름은 아래 다이어그램으로 확인할 수 있다.

![cvmm process lifecycle](docs/diagrams/process-lifecycle-cleanup.svg)

![cvmm client action dispatch](docs/diagrams/client-action-dispatch.svg)

## 문서

- [requirements](docs/requirements.md): 지원 범위와 manifest 계약
- [design](docs/design.md): load/start/client/console 설계
- [architecture](docs/architecture.md): package 책임과 데이터 흐름
- [operations](docs/operations.md): runbook과 검증/evidence 규칙
- [diagrams](docs/diagrams/README.md): Mermaid source와 SVG/PNG 산출물
- [benchmarks](docs/benchmarks.md): 현재 성능 측정 방침

## 개발 검증

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
