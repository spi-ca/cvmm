# 테스트 및 벤치마크 갭 보고

이 문서는 현재 `cvmm` 코드와 문서 기준으로 가능한 benchmark 후보와 누락된 테스트를 정리한다. 제거된 legacy ScreenFS artifact archive는 현재 `cvmm` evidence로 사용하지 않는다.

## 현재 검증 상태

현재 자동 테스트는 CLI 검증, entry dispatch, cloud-hypervisor client/수명주기, manifest/path 조립, util/sys helper까지 폭넓게 회귀를 막는다.

- `main_test.go` - usage 출력, invalid action, invalid `console-file` id 등 기본 CLI validation
- `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go` - YAML stdin/stdout 처리, malformed stdin 차단, Unix socket client dispatch
- `internal/hvm/client*_test.go` - action parsing, Unix socket transport, HTTP status mapping, redirect/dial failure
- `internal/hvm/hypervisor*_test.go`, `internal/hvm/node_checker_test.go`, `internal/hvm/load_more_test.go` - pidfile 검증, readiness/cancel, start/shutdown fallback, manifest/path/runas 조립
- `internal/model/api_test.go`, `internal/model/config*_test.go`, `internal/model/virtiofsd_args_test.go` - payload rendering, manifest decode, virtiofs args exact assertion
- `internal/util/*_test.go`, `internal/util/sys/*_test.go` - MAC/IEC/string/range helper, pidfile/process helper, wait/cancel 동작

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

### 최근 해소된 갭

다음 항목은 더 이상 우선 누락으로 보지 않는다.

- **CLI validation / client dispatch**: `main_test.go`, `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go`가 usage, invalid action/id, YAML decode/encode, malformed stdin 차단을 검증한다.
- **manifest/path/runas/initramfs**: `internal/hvm/hypervisor_test.go`, `internal/hvm/load_more_test.go`, `internal/model/config_more_test.go`가 initramfs missing/dir/stat error, absolute/relative disk·directory, generated tap/MAC, runas/socket-group 전파를 검증한다.
- **client dispatch/transport**: `internal/hvm/client_test.go`, `internal/hvm/client_more_test.go`, `internal/hvm/client_action_test.go`가 Unix socket HTTP method/path/status, invalid action parsing, dial failure, redirect 거부를 검증한다.
- **lifecycle/pidfile/shutdown/start**: `internal/hvm/hypervisor_shutdown_test.go`, `internal/hvm/hypervisor_start_more_test.go`, `internal/hvm/hypervisor_test.go`, `internal/hvm/node_checker_test.go`가 pidfile collision/cleanup, readiness cancel, `VmCreate`/`VmBoot` 실패, graceful power-button shutdown, API-not-ready 및 pre-boot rejection fallback을 검증한다.
- **virtiofs args / util assertions**: `internal/model/virtiofsd_args_test.go`, `internal/util/*_test.go`, `internal/util/sys/*_test.go`가 exact args, socket-group 분기, 값 파서/문자열 helper, pid/process wait helper를 assertion 기반으로 검증한다.

### Hypervisor/virtiofsd에 아직 남은 갭

대상: `internal/hvm/hypervisor.go`, `internal/model/virtiofsd_args.go`

남은 항목:

- 여러 share에 대한 `virtiofsdRecoiler` fan-out과 restart loop를 더 직접적으로 검증하는 테스트
- child stdout/stderr capture와 exit error formatting의 상세 assertion
- 실제 `cloud-hypervisor`/`virtiofsd` binary를 사용하는 privileged integration evidence

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

1. `console` / `console-file` / `util.OpenPty`의 PTY attach·cancel·path 계산 테스트 추가
2. `virtiofsdRecoiler`의 multi-share fan-out, restart loop, 종료 순서 assertion 보강
3. child stdout/stderr capture와 exit error formatting 상세 검증 추가
4. `start`/`shutdown`/`console` entrypoint의 실제 signal wiring을 subprocess 수준으로 확장
5. local `Benchmark*` 추가: config assembly, virtiofs args, fake socket client RTT

통합 host benchmark harness(`start`/readiness/virtiofsd fan-out/shutdown`)는 위 단위 테스트가 안정된 뒤 별도 작업으로 분리한다.

## 보고 시 주의사항

- benchmark 결과가 없으면 수치나 성능 개선을 주장하지 않는다.
- local unit benchmark와 privileged host benchmark를 같은 표에서 직접 비교하지 않는다.
- legacy ScreenFS artifact archive를 현재 `cvmm` 기준 evidence로 재사용하지 않는다.
- 통합 benchmark는 binary versions, manifest, image, command line, raw output, failure/timeout case를 함께 저장한다.
