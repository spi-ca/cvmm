# 테스트 및 벤치마크 갭 보고

이 문서는 현재 `cvmm` 코드와 문서 기준으로 가능한 benchmark 후보와 남은 테스트/운영 evidence 갭을 정리한다. 제거된 legacy non-cvmm artifact archive는 현재 `cvmm` evidence로 사용하지 않는다.

현재 자동화와 문서 설명의 network 기준선은 nested `net` + default `passt`이며, `net.backend: tap`은 명시적 호환 경로다.

## 현재 검증 상태

현재 자동 테스트는 CLI validation과 env binding, entry/client/console signal wiring, cloud-hypervisor client/수명주기, manifest/path 조립, duplicate virtio-fs basename 거부, console PTY happy-path, terminal resize/restore, poller/PTy cancel·HUP, virtiofsd multi-share restart/shutdown ordering을 회귀 보호한다. ADR-001 landing 이후에는 아래 network/backend 경계도 `go test ./...`에 포함된다.

- `main_test.go`, `docs_smoke_test.go` - usage 출력, invalid action, env override/binding, flag > env precedence, markdown local link 해석, core docs/`.pi` inventory smoke 검증
- `internal/model/config_network_test.go`, `internal/model/config*_test.go` - default `passt`, explicit `tap`, legacy top-level field 병합, `net.if_name` without `net.backend: tap` rejection, vhost-user payload 조립 검증
- `internal/hvm/passt_more_test.go` - TAP 전용 `CAP_NET_ADMIN`, `passt` 무-ambient-capability, `passt` command shape(`--foreground`, no `--pid`), unsafe `run/` permission rejection, `passt.pid` 기록/정리, `vm.create` 이후 fatal exit cleanup 검증
- `internal/hvm/client*_test.go`, `internal/hvm/console_security_test.go`, `internal/hvm/hypervisor*_test.go`, `internal/hvm/load_more_test.go`, `internal/hvm/node_checker_test.go`, `internal/hvm/virtiofsd_recoiler_more_test.go` - action parsing, Unix socket transport, lifecycle/pidfile/readiness/cancel, manifest/path/runas 조립, `NODE_NAME` basename/path-traversal 차단, virtiofsd fan-out/restart/shutdown ordering, child stdout/stderr capture 및 `invoke` exit formatting 검증
- `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go`, `internal/entry/client_signal_test.go`, `internal/entry/console_file_test.go`, `internal/entry/signal_console_test.go` - YAML stdin/stdout 처리, malformed stdin 차단, Unix socket client dispatch, `start`/`shutdown`/`console`/`console-file` signal cancel, fake `VmInfo` + 실제 PTY attach 검증
- `internal/util/*_test.go`, `internal/util/sys/*_test.go` - 값 파서, PTY/terminal, pidfile/process helper, user/group lookup, Viper replacer, logging/lookup edge 검증

공식 benchmark harness는 아직 없지만, 로컬 `Benchmark*` 함수는 `internal/model/bench_test.go`, `internal/hvm/bench_test.go`, `internal/util/sys/bench_test.go`, `internal/util/pty_bench_test.go`에 있다.

guest memory THP adaptive handling은 구현되었다. 현재 테스트는 `internal/model`의 enabled/disabled/unset 렌더링, 기본 shared-memory path의 `mergeable=on` 생략, `internal/hvm`의 host probe fixture, start-path THP payload 선택/재시도, `client vm-create` wire body의 explicit `thp:false`를 회귀 보호한다. 구현 범위와 남은 운영 evidence는 [`adr/0002-adaptive-thp-handling.md`](adr/0002-adaptive-thp-handling.md)를 따른다.

## 가능한 benchmark

### 로컬 unit benchmark

실제 KVM, `cloud-hypervisor`, `virtiofsd`, `passt` 없이 추가 가능한 항목이다.

| 후보 | 대상 코드 | 측정값 | 비고 |
| --- | --- | --- | --- |
| manifest decode | `model.LoadConfig` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 |
| VM config assembly | `Config.VMConfig` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 |
| virtiofs config/args | `Config.VirtiofsConfig`, `VirtiofsConfig.CommandArgs` | ns/op, allocs/op | `internal/model/bench_test.go` 구현 |
| `hvm.Load` path assembly | `hvm.Load` | ns/op, allocs/op | `internal/hvm/bench_test.go` 구현 |
| API client encoding/decoding | fake Unix socket HTTP server against `clientImpl` | round-trip latency, allocs/op | `internal/hvm/bench_test.go` 구현 |
| PID helper | `sys.AcquirePidFile`, `ReadPidFile`, `IsPidFileActive` | ns/op | `internal/util/sys/bench_test.go` 구현 |
| PTY helper smoke | `util.OpenPty` with local PTY pair | cancel latency | `internal/util/pty_bench_test.go` 구현 |

권장 명령 예:

```bash
go test -bench=. -benchmem ./internal/model ./internal/hvm ./internal/util/...
```

### 로컬 smoke / regression benchmark

- `client` fake socket smoke: 각 `ClientAction`이 기대 method/path/body를 호출하는지 확인
- `console-file` PTY smoke: `/dev/pts/<id>` 연결과 cancel 동작 확인
- `systemd-analyze verify contrib/cvmm@.service`: 설치 host에서는 실제 service user/group과 실행 파일 경로 전제 필요
- `docker build --target build -f Dockerfile .`: build stage smoke

### 통합 / host benchmark

실제 host 권한과 binary가 있어야 한다.

| 후보 | 필요 환경 | 핵심 evidence |
| --- | --- | --- |
| `start NODE` end-to-end (`passt`) | `/dev/kvm`, images/nodes root, `cloud-hypervisor`, `virtiofsd`, `passt`, dedicated non-root service user/group | command, manifest, image, versions, repeated samples |
| `start NODE` end-to-end (`tap`) | `/dev/kvm`, `/dev/net/tun`, tap 준비, 필요 capability | command, manifest, image, versions, repeated samples |
| API readiness | 실제 `cloud-hypervisor` API socket | spawn 시각, first `vmm.ping` 시각, timeout |
| `passt` readiness | 실제 `passt.sock` | helper spawn 시각, socket ready 시각, failure cleanup |
| `vm.create`/`vm.boot` latency | valid image/manifest | API call timestamps, guest state |
| `virtiofsd` fan-out | `directory[]` N개 fixture | helper count, socket ready time, RSS/fd count |
| `client vm-info/vm-counters` RTT | running VM | repeated round-trip latency |
| `console NODE` attach | running VM with PTY console | attach latency, cancel latency |
| `shutdown NODE` | running VM/manager pidfile | graceful time, forced kill fallback 여부 |
| systemd start/stop | installed unit and binary | `systemctl` timestamps, journal excerpts |

통합 benchmark 착수 전 최소 확인:

```bash
command -v cloud-hypervisor virtiofsd passt
cloud-hypervisor --version
virtiofsd --version
passt --version || passt --help | head -n 1
ls -ld /srv/vmm/images /srv/vmm/nodes
ls -l /dev/kvm /dev/net/tun
id
```

## 모듈별 단위 테스트 매트릭스

통합 환경 없이 `go test ./...` 안에서 우선 유지·확장할 단위 테스트는 아래처럼 나눈다. 외부 `cloud-hypervisor`, `virtiofsd`, `passt`, `/dev/kvm`, systemd 실행은 사용하지 않고 fake, tempdir, stub executable, local Unix socket/PTY만 사용한다.

| 모듈 | 핵심 로직 | 단위 테스트 후보 | test double |
| --- | --- | --- | --- |
| `main` | CLI action/argument validation, flag/env binding | missing args, invalid action, invalid `console-file` id, usage stderr, env key mapping/override, flag/env precedence | subprocess helper 또는 `exec.Command(go test helper)` |
| `internal/entry` | command entrypoint wiring, stdin/stdout YAML, signal cancel | `Client` bodyful action decode/encode, invalid YAML, `console-file` path construction, `client`/`start`/`shutdown`/`console`/`console-file` signal cancel path | fake `hvm.Client`, temp stdin/stdout, context/signal helper, subprocess |
| `internal/hvm` | `Load`, client action parsing, API client, lifecycle coordination | path resolution table, runas failure, `passt` permission/run-dir rejection, readiness retry timeout, pidfile collision, fatal helper exit, console path validation | tempdir manifests, fake Unix socket server, fake client, stub executable |
| `internal/model` | manifest schema, VM payload, command arg rendering, enum parsing | network defaulting, absolute/relative `disk`/`directory`, initramfs missing, vhost-user vs tap rendering, virtiofs exact args | pure table tests/golden strings |
| `internal/util` | value parsing/rendering, terminal helpers, format helpers | `ExecutionResult`, log trimming, `LookupBinary`, `PFlagViperReplacer`, escape sequence, terminal resize/restore, PTY close/cancel | table tests, `io.Pipe`, local PTY, subprocess where terminal state 필요 |
| `internal/util/sys` | pidfile/process/user/platform helpers | pidfile acquire/active/cleanup, invalid pid content, user/group lookup error path | tempdir pidfiles, current process pid, nonexistent user/group/id |

## 최근 해소된 갭

다음 항목은 더 이상 주요 gap으로 보지 않는다.

- **network backend 전환 핵심 단위 테스트**: `internal/model/config_network_test.go`, `internal/hvm/passt_more_test.go`가 default `passt`, explicit `tap`, TAP-only `if_name` rejection, vhost-user payload, helper lifecycle, pid ownership, capability gating, unsafe runtime-dir rejection을 검증한다.
- **CLI validation / client dispatch / env binding**: `main_test.go`, `internal/entry/client_dispatch_test.go`, `internal/entry/client_test.go`, `internal/entry/client_signal_test.go`가 usage, invalid action/id, YAML decode/encode, malformed stdin 차단, env override, flag > env precedence, in-flight `client` request cancel path를 검증한다.
- **entry signal wiring / console attach**: `internal/entry/signal_console_test.go`가 `shutdown`/`console`/`console-file` subprocess signal cancel, `console` fake `VmInfo` + 실제 PTY attach, `console-file` attach 및 missing-PTY panic 경로를 검증한다.
- **manifest/path/runas/initramfs**: `internal/hvm/hypervisor_test.go`, `internal/hvm/load_more_test.go`, `internal/model/config_more_test.go`가 initramfs missing/dir/stat error, absolute/relative disk·directory, duplicate virtio-fs basename 거부, generated TAP name/MAC, runas/socket-group 전파, invalid `NODE_NAME` 거부를 검증한다.
- **virtiofsd / invoke 세부 assertion**: `internal/hvm/virtiofsd_recoiler_more_test.go`가 multi-share fan-out, restart loop, shutdown ordering, child stdout/stderr capture, `invoke` exit formatting을 검증한다.

## 아직 남은 갭 / blocker

### Privileged integration evidence

다음 항목은 실제 host 권한, 설치된 binary, 또는 런타임 fixture가 있어야 해서 여전히 별도 작업이다.

- 실제 `cloud-hypervisor`/`virtiofsd`/`passt` binary와 `/dev/kvm`, image/node root를 사용한 default `passt` 경로의 `start`/readiness/boot/shutdown end-to-end evidence
- 실제 `/dev/net/tun`과 tap 준비 상태를 사용한 explicit `tap` 경로의 `start`/readiness/boot/shutdown end-to-end evidence
- running VM 기준 `client vm-info/vm-counters` RTT, `console NODE` attach latency, `virtiofsd` fan-out scaling, systemd start/stop timings

### Service / packaging verification blocker

- `systemd-analyze verify contrib/cvmm@.service`는 host에 실제 실행 파일 경로와 service user/group이 준비되어 있어야 의미 있는 결과를 남길 수 있다. 로컬 개발 환경에서 `/usr/bin/cvmm` 미설치 또는 placeholder service account 부재면 검증 결과가 환경 blocker에 묶인다.

### 운영 evidence gap

- default `passt` 배포에서 dedicated non-root service user가 소유한 `<node>/run/`을 실제로 어떻게 provisioning하는지에 대한 host-level evidence가 아직 없다.
- privileged host에서 실제 THP 지원/비지원 커널별 final decision log와 `vm.create` memory JSON을 수집한 운영 evidence는 아직 없다.
- TAP compatibility 운영 문서와 default `passt` 운영 문서의 실측 비교표도 아직 없다.

## 후속 우선순위 제안

1. privileged host fixture를 준비해 default `passt`와 explicit `tap` 경로를 분리한 integration evidence를 수집하기
2. host-installed binary/service user가 있는 환경에서 `systemd-analyze verify contrib/cvmm@.service` 결과를 다시 기록하기
3. `passt` 기본 경로와 `tap` 호환 경로의 startup/shutdown/RTT benchmark를 같은 host 조건에서 분리 측정하기
4. 필요 시 dedicated service user/group과 `<node>/run/` provisioning 절차를 별도 운영 evidence로 승격하기

## 보고 시 주의사항

- benchmark 결과가 없으면 수치나 성능 개선을 주장하지 않는다.
- local unit benchmark와 privileged host benchmark를 같은 표에서 직접 비교하지 않는다.
- legacy non-cvmm artifact archive를 현재 `cvmm` 기준 evidence로 재사용하지 않는다.
- 통합 benchmark는 binary versions, manifest, image, command line, raw output, failure/timeout case를 함께 저장한다.
