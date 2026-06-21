# cvmm 설계

이 문서는 `cvmm`가 node manifest를 읽어 `cloud-hypervisor`/`passt`/`virtiofsd` 런타임으로 바꾸는 방식을 설명한다.

## 1. 설정 계층

설정 입력은 세 층이다.

1. `main.go`의 기본 플래그 값
2. 같은 이름의 environment variable (`IMAGE_ROOT`, `NODE_ROOT` 등)
3. CLI 플래그

구현은 `pflag` + `viper` 조합이며, `-`와 `_`는 내부적으로 `.` key로 정규화한다.

## 2. 노드 로드

`internal/hvm.Load()`는 아래를 수행한다.

1. `<node-root>/<node>`와 `<node-root>/<node>/<volatile-directory>`를 계산한다.
2. pid, API socket, `passt.sock`, `passt.pid`, virtiofs socket, virtiofs helper pid 경로를 만든다.
3. `config.yaml`을 읽어 `model.Config`로 decode한다.
4. `model.Config.NormalizeNetwork()`로 legacy top-level `net_mac_addr`/`net_if_name`를 nested `net`으로 병합하고 backend 기본값을 `passt`로 채운다.
5. `<image-root>/<image>` 아래의 `vmlinuz`, `initramfs.img`, `root.img`를 resolve한다.
6. 비어 있는 `net.mac_addr`를 보완하고, TAP backend일 때만 비어 있는 `net.if_name`를 생성한다.
7. manifest를 `model.VmConfig`와 `[]model.VirtiofsConfig`로 변환한다.

`Hypervisor.Start()` 시작 gate는 모든 backend에서 `<node-root>/<node>/run/` symlink를 거부한다. `passt` backend의 dedicated non-root service uid/gid, `--runas` 제한, `run/` owner/mode 검사는 그 다음 start-time 검증으로 수행한다. 그래서 `client`, `console`, `shutdown`처럼 VM API socket이나 manager pidfile을 대상으로 하는 비기동 명령은 manifest load만으로 이 start-time 배포 검증에 막히지 않는다.

핵심 network 규칙:

- `net.backend`가 비어 있으면 `passt`
- `net.if_name`은 TAP 전용
- `net.if_name` 또는 legacy `net_if_name`에 값이 있으면 `net.backend: tap`이 필요
- root manager + `--runas`는 `passt` backend에서 거부

## 3. VM 기동 흐름

`start NODE`의 핵심 흐름:

1. signal-aware context 생성
2. 모든 backend에서 `run/` directory symlink 거부
3. `passt` backend면 service uid/gid와 `run/` owner/mode 검증
4. top-level pid file 확보
5. `cloud-hypervisor --api-socket path=...` 프로세스 시작
6. `passt` backend면 `passt --vhost-user --socket <node-run>/passt.sock --foreground` helper도 시작
7. `vmm.ping` 성공까지 대기
8. `passt.sock` readiness 대기
9. `vm.create` 호출
10. background `virtiofsd` reconciler 시작
11. `vm.boot` 호출
12. 부모 context가 끝날 때까지 상태 감시
13. 종료 시 power-button 또는 프로세스 종료로 정리

이 설계는 VM spec 생성과 프로세스 orchestration을 분리한다.

- spec 생성: `internal/model`
- process/API orchestration: `internal/hvm`
- CLI action entry: `internal/entry`

## 4. 네트워크 모델

manifest-managed NIC는 첫 구현에서도 하나만 다룬다.

- 기본 backend는 `passt`
- TAP 호환 경로는 `net.backend: tap`
- `passt`에서는 cloud-hypervisor network payload가 `vhost_user`, `vhost_socket`, `vhost_mode: "Client"`를 사용한다.
- TAP에서는 기존처럼 `tap=<ifname>` payload를 사용한다.
- `CAP_NET_ADMIN`은 TAP backend일 때만 `cloud-hypervisor` child에 붙는다.

`passt`는 share fan-out helper가 아니라 node-scoped 단일 helper다. `vm.create` 이후 비정상 종료는 transparent reconnect 대신 fatal error로 처리하고 기존 shutdown/cleanup 경로로 수습한다.

## 5. virtio-fs 모델

manifest의 `directory` 배열은 두 군데에 반영된다.

- guest 쪽에는 `VmConfig.Fs` 항목으로 들어간다.
- host 쪽에는 항목별 `virtiofsd` 프로세스로 실행된다.

socket 파일명과 pid 파일명은 각각의 템플릿에서 디렉터리 basename suffix를 붙여 만든다. guest tag도 같은 basename을 쓴다. `virtiofsd` 자체에 pid 옵션을 넘기는 방식이 아니라, `cvmm`가 helper process 시작 후 pid를 share별 `virtiofs*.pid`에 기록하고 helper 종료 시 정리한다. basename이 같은 `directory` 항목은 tag/socket/pid 충돌을 만들기 때문에 로드 단계에서 거부한다.

## 6. console 모델

- `start --console`은 VM console을 stdio로 직접 붙이는 cloud-hypervisor config를 만든다.
- `console NODE`는 cloud-hypervisor API에서 PTY 경로를 읽어 attach한다.
- `console-file PTY_ID`는 `/dev/pts/<id>`를 직접 연다. 이 명령은 trusted-admin 용도이며, 비-root 실행 시 현재 euid가 소유한 PTY만 허용한다.

PTY attach 로직은 `internal/util.OpenPty()`에 모여 있다.

## 7. client 모델

`client ACTION NODE`는 노드의 API socket에 대해 local HTTP over UNIX socket 호출을 수행한다.

- 조회 action: `vmm-ping`, `vm-info`, `vm-counters` 등
- 상태 변경 action: `vm-boot`, `vm-shutdown`, `vm-reboot` 등
- body 필요 action: `vm-create`, `vm-resize`, `vm-add-disk`, `vm-snapshot` 등

body 필요 action은 YAML을 stdin에서 읽는다. 따라서 docs와 tooling은 JSON이 아니라 YAML request 예제를 기준으로 맞추는 편이 안전하다.

## 8. `passt` lifecycle과 권한 경계

- binary discovery는 `--passt-path`/`PASST_PATH`를 따른다.
- helper command shape는 `--foreground`를 포함하고 `--pid`는 포함하지 않는다.
- `cvmm`가 direct child PID를 `<node-run>/passt.pid`에 기록하고 종료 시 정리한다.
- readiness는 pidfile이 아니라 `passt.sock` Unix socket 상태로 확인한다.
- 모든 backend에서 `<node-root>/<node>/run/` symlink는 거부된다.
- `passt` backend에서 `<node-root>/<node>/run/`은 서비스 uid 소유여야 하고 mode가 `0700`보다 느슨하면 `start`가 실패한다.
- `--runas`는 계속 `cloud-hypervisor` child 전용이며, service uid/gid와 다른 값으로 바꾸는 조합은 `passt` backend에서 지원하지 않는다.

## 9. 실패와 정리

- pid file이 active면 중복 기동을 거부한다.
- API readiness 대기 실패 시 start는 실패한다.
- `passt` readiness 실패, `vm.create` 실패, context cancellation, helper 비정상 종료 시 `passt.sock`/`passt.pid`를 포함한 ancillary cleanup을 수행한다.
- `shutdown`은 SIGTERM 후 제한 시간 동안 종료를 기다리고, 필요 시 kill로 정리한다.
- `virtiofsd`는 circuit breaker 기반 recoiler로 반복 실패를 완화하며, 실행 중 helper pid file을 유지하고 종료 후 제거한다.

## 10. 문서화 주의점

- 현재 저장소의 active design은 Go/cloud-hypervisor/`passt`/`virtiofsd`다.
- 기본 manifest-managed network path는 `passt`이며, TAP은 explicit compatibility backend로만 설명한다.
- guest memory THP adaptive handling은 구현되었다. 현재 코드는 load 시 THP를 미결정 상태로 두고, `start` 직전 Linux THP/shmem policy와 `PR_GET_THP_DISABLE`를 probe해 최종 `vm.create` payload에 `thp=on` 또는 explicit `thp=off`/`thp:false`를 반영한다. 기본 shared-memory payload는 `shared=on`을 유지하되 cloud-hypervisor v52 호환성을 위해 default `mergeable=on`을 보내지 않는다. THP 관련 `vm.create` 실패는 boot 전 1회 `thp` disabled payload로만 재시도하며, 세부 결정은 [`adr/0002-adaptive-thp-handling.md`](adr/0002-adaptive-thp-handling.md)에 기록한다.
- 제거된 legacy non-cvmm artifact archive를 현재 설계 근거로 섞지 않는다.
