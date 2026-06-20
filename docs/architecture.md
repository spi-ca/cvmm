# cvmm 아키텍처

## 시스템 컨텍스트

```text
node config.yaml + image directory
          |
          v
        cvmm
          |
          +--> cloud-hypervisor (API socket, VM lifecycle)
          |
          +--> virtiofsd per shared directory
          |
          +--> console/client helpers
```

## 패키지 구조

| 경로 | 책임 |
| --- | --- |
| `main` | CLI, flags, Viper binding, action dispatch |
| `internal/entry` | `start`, `shutdown`, `console`, `console-file`, `client` 진입점 |
| `internal/hvm` | hypervisor load/start/shutdown, API client, virtiofsd reconciler |
| `internal/model` | manifest schema, cloud-hypervisor request/response types, command-line rendering |
| `internal/util` | 로그, PTY, PID file, 바이너리 탐색, 범용 helper |
| `internal/util/sys` | build-tag별 OS/syscall helper |
| `contrib/cvmm@.service` | systemd 서비스 예시 |

## 데이터 흐름

### Start path

```text
CLI flags/env
  -> entry.Start
  -> hvm.Load
  -> model.LoadConfig(config.yaml)
  -> model.Config -> model.VmConfig + []model.VirtiofsConfig
  -> Hypervisor.Start
  -> cloud-hypervisor API calls (ping/create/boot)
```

### Client path

```text
CLI action
  -> entry.Client
  -> hvm.Load (socket path resolution)
  -> hvm.Client over UNIX socket HTTP
  -> YAML stdin decode for bodyful actions
```

### Console path

```text
entry.Console
  -> VmInfo lookup
  -> PTY path extraction
  -> util.OpenPty
```

## 런타임 파일

노드별 runtime 파일은 보통 `<node-root>/<node>/run/` 아래에 생긴다.

- `cvmm.pid`
- `cloudhypervisor.pid`
- `cloudhypervisor.sock`
- `virtiofs*.sock`

문서나 도구는 이 위치를 hard-code하기보다 flag 기반 경로 계산을 따라야 한다.

## 주석 커버리지 경계

주석 정책은 [`commenting.md`](commenting.md)를 따른다. active Go component는 `main`, `internal/entry`, `internal/hvm`, `internal/model`, `internal/util`, `internal/util/sys`이며, 각 package comment와 non-test top-level function/method/struct 및 중요한 exported 계약 type이 audit 대상이다.

## 운영 경계

- 서비스 예시는 systemd에서 `cvmm start %i`를 실행한다.
- `--runas` credential 전환은 현재 `cloud-hypervisor` 자식 프로세스에 적용된다.
- `virtiofsd` helper는 서비스 계정과 그 계정의 capability로 실행되며, `--runas` 사용자의 primary group이 `--socket-group`으로 전달될 수 있다.
- manifest `directory`는 절대경로도 허용하고 `virtiofsd`는 `--announce-submounts`를 사용하므로, 공유 디렉터리와 하위 mount 노출은 manifest 작성 권한과 서비스 계정/capability 모델에 좌우된다.
- 실제 권한 요구사항은 tap, socket, shared directory, ambient capability 설정 가능 여부에 좌우된다.

## 테스트 표면

현재 저장소 테스트는 주로 아래를 검증한다.

- manifest/model serialization
- cloud-hypervisor request model formatting
- client/model utility helpers
- 일부 hypervisor helper 동작

문서에서 현재 `cvmm` 구조와 무관한 레거시 테스트 명령이나 구조를 현재 아키텍처처럼 설명하지 않는다.
