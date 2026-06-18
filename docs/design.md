# cvmm 설계

이 문서는 `cvmm`가 node manifest를 읽어 `cloud-hypervisor`/`virtiofsd` 런타임으로 바꾸는 방식을 설명한다.

## 1. 설정 계층

설정 입력은 세 층이다.

1. `main.go`의 기본 플래그 값
2. 같은 이름의 environment variable (`IMAGE_ROOT`, `NODE_ROOT` 등)
3. CLI 플래그

구현은 `pflag` + `viper` 조합이며, `-`와 `_`는 내부적으로 `.` key로 정규화한다.

## 2. 노드 로드

`internal/hvm.Load()`는 아래를 수행한다.

1. `<node-root>/<node>`와 `<node-root>/<node>/<volatile-directory>`를 계산한다.
2. pid, API socket, virtiofs socket 경로를 만든다.
3. `config.yaml`을 읽어 `model.Config`로 decode한다.
4. `<image-root>/<image>` 아래의 `vmlinuz`, `initramfs.img`, `root.img`를 resolve한다.
5. 비어 있는 `net_mac_addr`, `net_if_name`를 보완한다.
6. manifest를 `model.VmConfig`와 `[]model.VirtiofsConfig`로 변환한다.

## 3. VM 기동 흐름

`start NODE`의 핵심 흐름:

1. signal-aware context 생성
2. top-level pid file 확보
3. `cloud-hypervisor --api-socket path=...` 프로세스 시작
4. `vmm.ping` 성공까지 대기
5. `vm.create` 호출
6. background `virtiofsd` reconciler 시작
7. `vm.boot` 호출
8. 부모 context가 끝날 때까지 상태 감시
9. 종료 시 power-button 또는 프로세스 종료로 정리

이 설계는 VM spec 생성과 프로세스 orchestration을 분리한다.

- spec 생성: `internal/model`
- process/API orchestration: `internal/hvm`
- CLI action entry: `internal/entry`

## 4. virtio-fs 모델

manifest의 `directory` 배열은 두 군데에 반영된다.

- guest 쪽에는 `VmConfig.Fs` 항목으로 들어간다.
- host 쪽에는 항목별 `virtiofsd` 프로세스로 실행된다.

socket 파일명은 템플릿에서 디렉터리 basename suffix를 붙여 만든다. 따라서 디렉터리 이름 충돌은 운영자가 피해야 한다.

## 5. console 모델

- `start --console`은 VM console을 stdio로 직접 붙이는 cloud-hypervisor config를 만든다.
- `console NODE`는 cloud-hypervisor API에서 PTY 경로를 읽어 attach한다.
- `console-file PTY_ID`는 `/dev/pts/<id>`를 직접 연다.

PTY attach 로직은 `internal/util.OpenPty()`에 모여 있다.

## 6. client 모델

`client ACTION NODE`는 노드의 API socket에 대해 local HTTP over UNIX socket 호출을 수행한다.

- 조회 action: `vmm-ping`, `vm-info`, `vm-counters` 등
- 상태 변경 action: `vm-boot`, `vm-shutdown`, `vm-reboot` 등
- body 필요 action: `vm-create`, `vm-resize`, `vm-add-disk`, `vm-snapshot` 등

body 필요 action은 YAML을 stdin에서 읽는다. 따라서 docs와 tooling은 JSON이 아니라 YAML request 예제를 기준으로 맞추는 편이 안전하다.

## 7. 실패와 정리

- pid file이 active면 중복 기동을 거부한다.
- API readiness 대기 실패 시 start는 실패한다.
- `shutdown`은 SIGTERM 후 제한 시간 동안 종료를 기다리고, 필요 시 kill로 정리한다.
- `virtiofsd`는 circuit breaker 기반 recoiler로 반복 실패를 완화한다.

## 8. 문서화 주의점

- 현재 저장소의 active design은 Go/cloud-hypervisor/virtiofsd다.
- `docs/artifacts/**`의 copied legacy 자료를 현재 설계 근거로 섞지 않는다.
