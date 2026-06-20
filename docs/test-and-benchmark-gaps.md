# 테스트 및 벤치마크 갭 보고

이 문서는 현재 `cvmm` 코드와 문서 기준으로 가능한 benchmark 후보와 누락된 테스트를 정리한다. 제거된 legacy non-cvmm artifact archive는 현재 `cvmm` evidence로 사용하지 않는다.

## 현재 검증 상태

현재 자동 테스트는 CLI validation과 env binding, entry/client/console signal wiring, cloud-hypervisor client/수명주기, manifest/path 조립, duplicate virtio-fs basename 거부, console PTY happy-path, terminal resize/restore, poller/PTy cancel·HUP, virtiofsd multi-share restart/shutdown ordering, 그리고 util/model/sys edge helper 상당수를 회귀 보호한다. 남는 갭은 주로 privileged host integration evidence와 host-installed binary/tooling 전제다.

- `main_test.go`, `docs_smoke_test.go` - usage 출력, invalid action, env override/binding, flag > env precedence, markdown local link 해석, core docs/`.pi` inventory smoke 검증
- `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go`, `internal/entry/client_signal_test.go`, `internal/entry/console_file_test.go`, `internal/entry/signal_console_test.go` - YAML stdin/stdout 처리, malformed stdin 차단, Unix socket client dispatch, `start`/`shutdown`/`console`/`console-file` signal cancel, fake `VmInfo` + 실제 PTY attach, `console-file` missing-PTY panic 경로 검증
- `internal/hvm/client*_test.go`, `internal/hvm/console_security_test.go`, `internal/hvm/hypervisor*_test.go`, `internal/hvm/node_checker_test.go`, `internal/hvm/load_more_test.go`, `internal/hvm/virtiofsd_recoiler_more_test.go` - action parsing, Unix socket transport, lifecycle/pidfile/readiness/cancel, manifest/path/runas 조립, `NODE_NAME` basename/path-traversal 차단, virtiofsd fan-out/restart/shutdown ordering, child stdout/stderr capture 및 `invoke` exit formatting 검증
- `internal/model/api_test.go`, `internal/model/config*_test.go`, `internal/model/virtiofsd_args_test.go`, `internal/model/enum_text_test.go` - payload rendering, manifest decode, virtiofs args exact assertion, enum invalid-text/zero-value marshal edge
- `internal/util/int_test.go`, `internal/util/mac_test.go`, `internal/util/str_test.go`, `internal/util/unit_test.go`, `internal/util/poller*_test.go`, `internal/util/execution_result_test.go`, `internal/util/key_seq_test.go`, `internal/util/log_test.go`, `internal/util/lookup_test.go`, `internal/util/pflag_viper_replacer_test.go`, `internal/util/pty_path_test.go`, `internal/util/terminal_test.go` - 값 파서, 문자열 helper, terminal raw-mode/resize/restore, poller rollback/add-remove/register edge, PTY open failure/cancel/HUP, non-terminal stdin 단일 copy 경로, direct `console-file` PTY ownership guard, exit/log/lookup/key-sequence/Viper/PTy-path edge
- `internal/util/sys/proc*_test.go`, `internal/util/sys/unfilemode_test.go`, `internal/util/sys/user_test.go` - pidfile/process helper, Linux `/proc` 기반 대기, mode helper, user/group lookup error path

공식 benchmark harness는 아직 없지만, 로컬 `Benchmark*` 함수는 `internal/model/bench_test.go`, `internal/hvm/bench_test.go`, `internal/util/sys/bench_test.go`에 추가되어 있다.

## 가능한 benchmark

### 로컬 unit benchmark

실제 KVM, `cloud-hypervisor`, `virtiofsd` 없이 추가 가능한 항목이다.

| 후보 | 대상 코드 | 측정값 | 비고 |
| --- | --- | --- | --- |
| manifest decode | `model.LoadConfig` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 (small/medium/large manifest fixture) |
| VM config assembly | `Config.VMConfig` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 |
| virtiofs config/args | `Config.VirtiofsConfig`, `VirtiofsConfig.CommandArgs` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 |
| `hvm.Load` path assembly | `hvm.Load` | ns/op, allocs/op | `internal/hvm/bench_test.go` 구현 (benchmark 중 project log는 discard) |
| API client encoding/decoding | `clientImpl` against fake Unix socket HTTP server | round-trip latency, allocs/op | `internal/hvm/bench_test.go` fake socket RTT 구현 |
| PID helper | `sys.AcquirePidFile`, `ReadPidFile`, `IsPidFileActive` | ns/op | `internal/util/sys/bench_test.go` 구현 |
| PTY helper smoke | `util.OpenPty` with local PTY pair | cancel latency | `internal/util/pty_bench_test.go` 구현 |

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
| `main` | CLI action/argument validation, flag/env binding | missing args, invalid action, invalid `console-file` id, usage stderr, env key mapping/override, flag/env precedence | subprocess helper 또는 `exec.Command(go test helper)` |
| `internal/entry` | command entrypoint wiring, stdin/stdout YAML, signal cancel | `Client` bodyful action decode/encode, invalid YAML, `console-file` path construction, `client`/`start`/`shutdown`/`console`/`console-file` signal cancel path | fake `hvm.Client`, temp stdin/stdout, context/signal helper, subprocess |
| `internal/hvm` | `Load`, client action parsing, API client, lifecycle coordination | path resolution table, runas failure, client action invalids, Unix socket HTTP status handling, readiness retry timeout, pidfile collision, console path validation | tempdir manifests, fake Unix socket server, fake client, stub executable |
| `internal/model` | manifest schema, VM payload, command arg rendering, enum parsing | `Config` defaults, absolute/relative `disk`/`directory`, initramfs missing, `--net` rendering, virtiofs exact args, enum invalid-text unmarshal and zero-value marshal paths | pure table tests/golden strings |
| `internal/util` | value parsing/rendering, terminal helpers, format helpers | `ExecutionResult`, log trimming, `LookupBinary`, `PFlagViperReplacer`, escape sequence, terminal resize/restore, PTY close/cancel | table tests, `io.Pipe`, local PTY, subprocess where terminal state 필요 |
| `internal/util/sys` | pidfile/process/user/platform helpers | pidfile acquire/active/cleanup, invalid pid content, user/group lookup error path, unsupported platform behavior where build tags allow | tempdir pidfiles, current process pid, nonexistent user/group/id, small helper process |

## 누락된 테스트

### 최근 해소된 갭

다음 항목은 더 이상 우선 누락으로 보지 않는다.

- **CLI validation / client dispatch / env binding**: `main_test.go`, `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go`, `internal/entry/client_signal_test.go`가 usage, invalid action/id, YAML decode/encode, malformed stdin 차단, env override, flag > env precedence, in-flight `client` request cancel path를 검증한다.
- **entry signal wiring / console attach**: `internal/entry/signal_console_test.go`가 `shutdown`/`console`/`console-file` subprocess signal cancel, `console` fake `VmInfo` + 실제 PTY attach, `console-file` attach 및 missing-PTY panic 경로를 검증한다. `start` signal wiring도 같은 helper로 추가되었고 ambient-capability exec가 허용된 환경에서 실행되며, 그렇지 않으면 실제 런타임 전제를 이유로 skip 한다.
- **manifest/path/runas/initramfs**: `internal/hvm/hypervisor_test.go`, `internal/hvm/load_more_test.go`, `internal/model/config_more_test.go`가 initramfs missing/dir/stat error, absolute/relative disk·directory, duplicate virtio-fs basename 거부, generated tap/MAC, runas/socket-group 전파, invalid `NODE_NAME` 거부를 검증한다.
- **client dispatch/transport**: `internal/hvm/client_test.go`, `internal/hvm/client_more_test.go`, `internal/hvm/client_action_test.go`가 Unix socket HTTP method/path/status, invalid action parsing, dial failure, redirect 거부를 검증한다.
- **lifecycle/pidfile/shutdown/start**: `internal/hvm/hypervisor_shutdown_test.go`, `internal/hvm/hypervisor_start_more_test.go`, `internal/hvm/hypervisor_test.go`, `internal/hvm/node_checker_test.go`가 pidfile collision/cleanup, readiness cancel, `VmCreate`/`VmBoot` 실패, graceful power-button shutdown, API-not-ready 및 pre-boot rejection fallback을 검증한다.
- **terminal / poller / PTY 로컬 갭**: `internal/entry/console_file_test.go`, `internal/util/terminal_test.go`, `internal/util/poller_test.go`, `internal/util/poller_edge_test.go`, `internal/util/pty_path_test.go`, `internal/util/execution_result_test.go`가 `PrepareTerminal` resize/restore/raw-mode 실패, poller rollback/add-remove/register edge, `OpenPty` open failure/cancel/HUP와 non-terminal pipe input integrity, direct `console-file` PTY ownership guard, validation 이후 `OpenPty` 실패 panic, `ExecutionResult` 일반 non-exit error branch를 검증한다.
- **virtiofsd / invoke 세부 assertion**: `internal/hvm/virtiofsd_recoiler_more_test.go`가 multi-share fan-out, restart loop, shutdown ordering, child stdout/stderr capture, `invoke` exit formatting을 검증하고, `internal/hvm/hypervisor_test.go`가 helper pid file 생성/정리를 검증한다.
- **deployment/docs smoke**: `docs_smoke_test.go`가 markdown local link check와 core docs/`.pi` inventory smoke를 고정하고, `docker build --target build -f Dockerfile .`는 로컬에서 build-stage smoke를 통과했다.

### 아직 남은 갭 / blocker

#### Privileged integration evidence

다음 항목은 실제 host 권한, 설치된 binary, 또는 런타임 fixture가 있어야 해서 여전히 별도 작업이다.

- 실제 `cloud-hypervisor`/`virtiofsd` binary와 `/dev/kvm`, `/dev/net/tun`, image/node root를 사용한 `start`/readiness/boot/shutdown end-to-end evidence
- running VM 기준 `client vm-info/vm-counters` RTT, `console NODE` attach latency, `virtiofsd` fan-out scaling, systemd start/stop timings

#### Tooling / host prerequisite blocker

- `systemd-analyze verify contrib/cvmm@.service`는 로컬에서 실행했지만 `/usr/bin/cvmm`가 설치되어 있지 않아 `Command /usr/bin/cvmm is not executable: No such file or directory`로 실패했다. host-installed binary 또는 override path가 있는 환경에서 다시 기록해야 한다.

#### Optional local follow-up

- `console-file`은 이제 경로 canonicalization/char-device 검증에 더해 비-root 실행 시 current euid 소유 PTY만 허용한다. root/trusted-admin이 다른 사용자 PTY를 직접 여는 시나리오는 여전히 운영 통제로 다뤄야 한다.
- `console-file`의 "유효한 `/dev/pts/<id>` 검증 이후에만 발생하는 open 실패"는 `internal/entry/console_file_test.go`의 deterministic fixture로 회귀 보호한다.
- `model.LoadConfig` benchmark와 `util.OpenPty` cancel latency benchmark는 각각 `internal/model/bench_test.go`, `internal/util/pty_bench_test.go`에 구현되어 있다.

## 후속 우선순위 제안

1. privileged host fixture를 준비해 실제 `cloud-hypervisor`/`virtiofsd` binary 기준 integration evidence를 수집하기
2. host-installed `/usr/bin/cvmm` 또는 unit override path가 있는 환경에서 `systemd-analyze verify contrib/cvmm@.service` 결과를 다시 기록하기
3. 필요 시 `console-file` root/admin cross-owner attach 정책을 별도 운영 evidence로 승격하기

통합 host benchmark harness(`start`/readiness/virtiofsd fan-out/shutdown`)는 여전히 별도 작업으로 분리한다.

## 보고 시 주의사항

- benchmark 결과가 없으면 수치나 성능 개선을 주장하지 않는다.
- local unit benchmark와 privileged host benchmark를 같은 표에서 직접 비교하지 않는다.
- legacy non-cvmm artifact archive를 현재 `cvmm` 기준 evidence로 재사용하지 않는다.
- 통합 benchmark는 binary versions, manifest, image, command line, raw output, failure/timeout case를 함께 저장한다.
