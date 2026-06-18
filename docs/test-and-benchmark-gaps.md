# 테스트 및 벤치마크 갭 보고

이 문서는 현재 `cvmm` 코드와 문서 기준으로 가능한 benchmark 후보와 누락된 테스트를 정리한다. `docs/artifacts/**`의 legacy 자료는 현재 `cvmm` evidence로 사용하지 않는다.

## 현재 검증 상태

현재 자동 테스트는 주로 모델 렌더링과 일부 유틸리티를 검증한다.

- `internal/hvm/client_action_test.go` - client action 문자열 round-trip
- `internal/hvm/hypervisor_test.go` - manifest 기반 `hvm.Load` happy path
- `internal/model/api_test.go` - cloud-hypervisor payload/string rendering fixture
- `internal/model/config_test.go` - manifest YAML marshal fixture
- `internal/model/virtiofsd_args_test.go` - 현재는 출력 smoke 성격이며 assertion이 약함
- `internal/util/*_test.go`, `internal/util/sys/unfilemode_test.go` - 값 타입/문자열 helper 일부. 일부 테스트(`mac`, `unit`, `str`)는 출력 확인 성격이 강해 assertion 기반 회귀 방어가 약하다.

아직 `Benchmark*` 함수나 공식 benchmark harness는 없다.

## 가능한 benchmark

### 로컬 unit benchmark

실제 KVM, `cloud-hypervisor`, `virtiofsd` 없이 추가 가능한 항목이다.

| 후보 | 대상 코드 | 측정값 | 비고 |
| --- | --- | --- | --- |
| manifest decode | `model.LoadConfig` | ns/op, allocs/op | 다양한 manifest 크기 fixture 필요 |
| VM config assembly | `Config.VMConfig` | ns/op, allocs/op | disk/share 개수별 table benchmark |
| virtiofs config/args | `Config.VirtiofsConfig`, `VirtiofsConfig.CommandArgs` | ns/op, allocs/op | `directory[]` fan-out 크기별 측정 |
| `hvm.Load` path assembly | `hvm.Load` | ns/op, allocs/op | tempdir fixture, initramfs present/missing 분리 |
| API client encoding/decoding | `clientImpl` against fake Unix socket HTTP server | round-trip latency, allocs/op | network 없는 local socket server 사용 |
| PID helper | `sys.AcquirePidFile`, `ReadPidFile`, `IsPidFileActive` | ns/op | tempdir pidfile 기준 |
| PTY helper smoke | `util.OpenPty` with local PTY pair | attach/close latency | terminal 상태 영향 분리 필요 |

권장 명령 예:

```bash
go test -bench=. -benchmem ./internal/model ./internal/hvm ./internal/util/...
```

### 로컬 smoke / regression benchmark

반복 측정보다는 회귀 탐지에 가깝다.

- `client` fake socket smoke: 각 `ClientAction`이 기대 method/path/body를 호출하는지 확인
- `console-file` PTY smoke: `/dev/pts/<id>` 연결과 cancel 동작 확인
- `systemd-analyze verify contrib/cvmm@.service`: 설치 host에서는 `/usr/bin/cvmm` 존재 전제 필요
- `docker build --target build -f Dockerfile .`: build stage smoke

### 통합 / host benchmark

실제 host 권한과 binary가 있어야 한다.

| 후보 | 필요 환경 | 핵심 evidence |
| --- | --- | --- |
| `start NODE` end-to-end | `/dev/kvm`, `/dev/net/tun`, image/node root, `cloud-hypervisor`, `virtiofsd` | command, manifest, image, versions, repeated samples |
| API readiness | 실제 `cloud-hypervisor` API socket | spawn 시각, first `vmm.ping` 시각, timeout |
| `vm.create`/`vm.boot` latency | valid image/manifest | API call timestamps, guest state |
| `virtiofsd` fan-out | `directory[]` N개 fixture | helper count, socket ready time, RSS/fd count |
| `client vm-info/vm-counters` RTT | running VM | repeated round-trip latency |
| `console NODE` attach | running VM with PTY console | attach latency, cancel latency |
| `shutdown NODE` | running VM/manager pidfile | graceful time, forced kill fallback 여부 |
| systemd start/stop | installed unit and binary | `systemctl` timestamps, journal excerpts |

통합 benchmark 착수 전 최소 확인:

```bash
command -v cloud-hypervisor virtiofsd
cloud-hypervisor --version
virtiofsd --version
ls -ld /srv/vmm/images /srv/vmm/nodes
ls -l /dev/kvm /dev/net/tun
getent passwd hvm
id hvm
```

## 모듈별 단위 테스트 매트릭스

통합 환경 없이 `go test ./...` 안에서 우선 추가할 단위 테스트는 아래처럼 나눈다. 외부 `cloud-hypervisor`, `virtiofsd`, `/dev/kvm`, systemd 실행은 사용하지 않고 fake, tempdir, stub executable, local Unix socket/PTY만 사용한다.

| 모듈 | 핵심 로직 | 단위 테스트 후보 | test double |
| --- | --- | --- | --- |
| `main` | CLI action/argument validation, flag/env binding | missing args, invalid action, invalid `console-file` id, usage stderr, env key mapping | subprocess helper 또는 `exec.Command(go test helper)` |
| `internal/entry` | command entrypoint wiring, stdin/stdout YAML, signal cancel | `Client` bodyful action decode/encode, invalid YAML, `console-file` path construction, signal cancel path | fake `hvm.Client`, temp stdin/stdout, context/signal helper |
| `internal/hvm` | `Load`, client action parsing, API client, lifecycle coordination | path resolution table, runas failure, client action invalids, Unix socket HTTP status handling, readiness retry timeout, pidfile collision | tempdir manifests, fake Unix socket server, fake client, stub executable |
| `internal/model` | manifest schema, VM payload, command arg rendering, enum parsing | `Config` defaults, absolute/relative `disk`/`directory`, initramfs missing, `--net` rendering, virtiofs exact args, enum invalid paths | pure table tests/golden strings |
| `internal/util` | value parsing/rendering, terminal helpers, format helpers | MAC/IEC invalid inputs, `AppendFileSuffix`, range formatting, escape sequence, PTY close/cancel | table tests, `io.Pipe`, local PTY where available |
| `internal/util/sys` | pidfile/process/user/platform helpers | pidfile acquire/active/cleanup, invalid pid content, user lookup error path, unsupported platform behavior where build tags allow | tempdir pidfiles, current process pid, small helper process |

## 누락된 테스트

### CLI parsing and dispatch

대상: `main.go`

누락:

- argument 없는 실행이 usage를 출력하고 실패하는지
- 각 action의 필수 argument 개수 검증
- `console-file`의 invalid PTY id 처리
- `client`의 invalid action 처리
- env/flag binding key 변환(`image-root` ↔ `IMAGE_ROOT`) 검증
- `start`, `shutdown`, `console`, `client` entrypoint의 공통 signal/cancel wiring 검증

권장 방식: `go test`에서 subprocess로 `go run .` 또는 test binary helper를 실행해 stdout/stderr/exit code를 확인한다.

### Manifest and path handling

대상: `internal/hvm/load.go`, `internal/model/config.go`

누락:

- `initramfs.img` 없음/디렉터리일 때 initramfs를 비우는 동작
- `disk[]`, `directory[]`의 absolute path vs relative path 처리
- `directory[]` basename이 virtio-fs tag/socket suffix가 되는 규칙
- `net_mac_addr`, `net_if_name` 미지정 시 생성 규칙
- manifest 파일 없음/잘못된 YAML 오류 전파
- `--runas` user lookup 실패와 socket group lookup 실패

### cloud-hypervisor client and action dispatch

대상: `internal/hvm/client.go`, `internal/entry/client.go`, `internal/hvm/client-action.go`

누락:

- 각 client method의 HTTP method/path/status handling
- bodyful action의 YAML stdin decode 성공/실패
- response YAML stdout encoding
- `ClientActionNameOf`의 invalid input과 `UnmarshalText` 오류 경로
- Unix socket dial 실패/redirect 거부/response body error message

이미 추가된 round-trip 테스트는 action 매핑 회귀를 막지만, HTTP transport와 entry dispatch까지는 검증하지 않는다.

### Hypervisor lifecycle

대상: `internal/hvm/hypervisor.go`, `internal/hvm/node_checker.go`

누락:

- pidfile collision과 cleanup
- cloud-hypervisor readiness timeout/retry
- `VmCreate`/`VmBoot` 실패 전파
- parent context cancel 시 `VmPowerButton` 호출과 error aggregation
- child stdout/stderr capture와 exit error formatting
- `NodeStatusChecker`의 matching, non-matching, context cancel 경로

권장 방식: fake client와 stub executable을 사용한다. 실제 `cloud-hypervisor`는 integration test로 분리한다.

### virtiofsd helper behavior

대상: `internal/model/virtiofsd_args.go`, `internal/hvm/hypervisor.go`

누락:

- `VirtiofsConfig.CommandArgs` exact assertion
- `--socket-group` 포함/미포함 분기
- thread pool size가 manifest CPU 수를 따르는지
- `virtiofsdRecoiler`가 share 개수별 helper를 시작하는지
- helper exit 시 재시작/종료 동작
- context cancel 시 helper cleanup 완료 여부

현재 `internal/model/virtiofsd_args_test.go`는 출력만 하므로 assertion 기반 golden test로 바꾸는 것이 우선이다.

### Console and terminal helpers

대상: `internal/entry/console.go`, `internal/entry/console_file.go`, `internal/util/pty.go`, `internal/util/poller_*.go`

누락:

- `console`이 `VmInfo.Config.Console.File` 경로를 사용해 PTY를 여는지
- `console-file`이 `/dev/pts/<id>`를 정확히 계산하는지
- escape sequence/cancel/hup 처리
- poller add/remove/register edge case

### Deployment/docs smoke

대상: `contrib/cvmm@.service`, `Dockerfile`, active docs

누락:

- `systemd-analyze verify contrib/cvmm@.service` 결과 기록
- `docker build --target build -f Dockerfile .` build-stage smoke
- Markdown link check 또는 최소 local link grep
- docs inventory와 `.pi` inventory를 CI/검증 절차로 고정

## 단위 테스트 우선순위 제안

1. `VirtiofsConfig.CommandArgs` assertion test로 print-only smoke 제거
2. 출력 위주 util 테스트(`mac`, `unit`, `str`)를 assertion 기반 table test로 강화
3. `Config`/`hvm.Load` path edge case table test 추가
4. `ClientAction` invalid/error path와 `entry.Client` bodyful YAML decode test 추가
5. `start`/`shutdown` 등 entrypoint signal/cancel wiring test 추가
6. fake Unix socket 기반 `clientImpl` method/status test 추가
7. `Hypervisor.Start` readiness/error path를 fake executable/client로 분리 테스트
8. local `Benchmark*` 추가: config assembly, virtiofs args, fake socket client RTT

통합 host benchmark harness(`start`/readiness/virtiofsd fan-out/shutdown`)는 위 단위 테스트가 안정된 뒤 별도 작업으로 분리한다.

## 보고 시 주의사항

- benchmark 결과가 없으면 수치나 성능 개선을 주장하지 않는다.
- local unit benchmark와 privileged host benchmark를 같은 표에서 직접 비교하지 않는다.
- `docs/artifacts/**` legacy 자료를 현재 `cvmm` 기준 evidence로 재사용하지 않는다.
- 통합 benchmark는 binary versions, manifest, image, command line, raw output, failure/timeout case를 함께 저장한다.
