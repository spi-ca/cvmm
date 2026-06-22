# 운영 및 검증

## 1. 전제 조건

운영 환경은 아래를 만족해야 한다.

- `cloud-hypervisor` 설치
- `virtiofsd` 설치
- `passt` 설치
- image root와 node root 접근 가능
- pid/socket/runtime 파일을 생성할 권한
- 공통 runtime 권한 준비: `<node-root>/<node>/run/`을 `cvmm` manager/service uid 소유, mode `0700` 이하로 유지
- backend별 추가 권한 준비
  - 기본 `passt`: dedicated non-root service user/group
  - explicit `tap`: tap/network/capability 설정 권한

예시 systemd unit은 [`../contrib/cvmm@.service`](../contrib/cvmm@.service)를 따른다. 기본 `passt` 배포에서는 `cvmm` 자체를 dedicated non-root service user로 실행해야 하며, `--runas`는 비워 두는 쪽이 기준이다. `virtiofsd`와 `passt` helper는 별도 credential 전환 없이 서비스 계정으로 실행된다. `--runas`는 `cloud-hypervisor` 자식 프로세스 credential에만 적용되고, `passt` backend에서는 service uid/gid와 다른 값으로 바꾸는 조합을 지원하지 않는다. manifest `directory`는 절대경로도 허용되고 `virtiofsd`는 `--announce-submounts`를 사용하므로, 공유 디렉터리 권한은 서비스 계정/capability와 submount 노출까지, socket 접근 권한은 socket group까지 포함해 설계해야 한다.

공통 runtime directory 추가 점검:

- `<node-root>/<node>/run/` 소유자가 실제 `cvmm` manager/service uid인지
- `<node-root>/<node>/run/` mode가 `0700`보다 느슨하지 않은지
- `client`/`console`/`shutdown`이 socket/pid 접근 전에 동일한 `run/` 소유자/mode/symlink 검증을 수행하는지

`passt` backend 추가 점검:

- `passt.sock` 접근 제어가 shared human login group에 열리지 않는지
- `passt.pid`와 `passt.sock`가 start/shutdown 시 생성·정리되는지
- root manager + `--runas` 패턴이 아닌지

guest memory THP adaptive handling은 구현되었다. `start`는 Linux THP sysfs(`enabled`, `shmem_enabled`)와 `PR_GET_THP_DISABLE`를 probe해 shared guest memory용 THP를 최종 결정한다. probe가 unreadable/missing/malformed이면 startup을 hard-fail하지 않고 warning 후 explicit `thp:false`/`thp=off` payload를 사용한다. 기본 shared-memory payload는 `shared=on`을 유지하지만 cloud-hypervisor v52가 거부하는 `mergeable=on,shared=on` 조합을 피하기 위해 default `mergeable=on`은 생략한다. THP-enabled `vm.create`가 THP 관련 오류로 실패하면 boot 전 1회 disabled payload로만 재시도한다. 운영 시 최종 판단은 [`adr/0002-adaptive-thp-handling.md`](adr/0002-adaptive-thp-handling.md)의 decision log와 `vm.create` memory JSON 로그 기준으로 확인한다.

TAP compatibility 배포에서는 아래 경계를 분리해서 본다.

- `CAP_NET_ADMIN`은 TAP backend일 때만 필요
- `--runas`는 여전히 `cloud-hypervisor`만 낮춘다
- `virtiofsd` helper는 서비스 계정 권한을 유지한다
- 그래도 `<node-root>/<node>/run/` owner/mode 보호는 동일하게 필요하다

## 2. 표준 검증 명령

문서 변경 최소 확인:

```bash
go test ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
```

코드/주석 변경 시 추가 확인:

```bash
gofmt -w .
go vet ./...
go test ./...
```

주석 커버리지 audit이 필요하면 parser 기반 점검을 저장소 내부 스크립트로 고정하거나 사용자 전용 임시 파일을 만들어 실행한다. world-writable 고정 경로를 그대로 실행하지 않는다. 예:

```bash
audit_file="$(mktemp "${TMPDIR:-/tmp}/cvmm-comment-audit.XXXXXX.go")"
# write or copy the parser audit program into "$audit_file"
go run "$audit_file" ./...
rm -f "$audit_file"
```

현재 `passt` 구현과 관련해 `go test ./...`가 최소한 덮어야 하는 근거는 아래다.

- default `passt`와 explicit `tap` manifest/model 테스트
- `net.if_name` without `net.backend: tap` rejection 테스트
- `passt` startup ordering, socket readiness, post-create fatal exit cleanup 테스트
- `passt.pid`를 `cvmm`가 기록/정리하는 테스트
- TAP 전용 `CAP_NET_ADMIN`, `passt` 무-ambient-capability, unsafe `run/` permission rejection 테스트

특정 테스트만 다시 돌릴 때:

```bash
go test -run TestName ./internal/...
```

## 3. 기본 실행 runbook

### VM 시작

```bash
go run . start NODE_NAME
```

필요 시 경로/바이너리 override:

```bash
go run . \
  --image-root /srv/vmm/images \
  --node-root /srv/vmm/nodes \
  --cloudhypervisor-path /usr/bin/cloud-hypervisor \
  --virtiofsd-path /usr/lib/virtiofsd \
  --passt-path /usr/bin/passt \
  start NODE_NAME
```

### TAP compatibility로 시작

manifest에 `net.backend: tap`을 넣은 뒤 필요하면 `--runas`를 함께 쓴다.

```bash
go run . \
  --image-root /srv/vmm/images \
  --node-root /srv/vmm/nodes \
  --cloudhypervisor-path /usr/bin/cloud-hypervisor \
  --virtiofsd-path /usr/lib/virtiofsd \
  --runas hvm \
  start TAP_NODE_NAME
```

### VM 종료

```bash
go run . shutdown NODE_NAME
```

### 콘솔 연결

```bash
go run . console NODE_NAME
```

### PTY 직접 연결

```bash
go run . console-file PTY_ID
```

`console-file`은 host PTY를 직접 여는 trusted-admin 용도다. 비-root로 실행할 때는 현재 euid가 소유한 `/dev/pts/<id>`만 허용된다.

### cloud-hypervisor API 호출

조회 예:

```bash
go run . client vm-info NODE_NAME
```

stdin body가 필요한 예:

```bash
cat request.yaml | go run . client vm-resize NODE_NAME
```

## 4. 운영 점검 포인트

- `NODE_NAME`은 안전한 basename(`node-01`, `vm.test` 등)만 사용하고 `/`, `..`, 공백은 피한다.
- `<node-root>/<node>/config.yaml`이 실제 source of truth인지 확인
- `<node-root>/<node>/run/`의 pid/socket 파일 충돌 여부와 owner/mode(`<=0700`)를 확인
- 기본 backend면 `passt.sock`, `passt.pid`가 있는지와 접근 제어가 기대값인지 확인
- TAP backend면 실제 `net.if_name`와 TAP 준비 상태를 확인
- `directory` 항목 수와 `virtiofsd` 프로세스 수가 일치하는지 확인
- `image`가 실제 이미지 디렉터리를 가리키는지 확인
- console 문제 시 `vm-info` 응답의 PTY 경로를 먼저 확인
- 사고 조사 시 backend(`passt`/`tap`)와 `run/` 아래 helper socket/pid 증거를 함께 확인한다.

## 5. Evidence 규칙

- 현재 저장소에는 `cvmm`용으로 정리된 formal benchmark/evidence bundle이 없다.
- 이전 legacy non-cvmm artifact archive는 제거했으며 현재 `cvmm` 운영 근거가 아니다.
- 새 운영 증거를 남길 때는 사용한 manifest, 이미지, 명령, host 환경, stdout/stderr, 검증 시각을 같이 기록한다.
- systemd evidence를 남길 때는 `User=`/`Group=`, capability 경계, `run/` owner/mode, backend(`passt`/`tap`)를 함께 적는다.
- 주석 audit evidence에는 대상 package, 누락/보강 항목, 사용한 parser 기반 점검 명령 또는 수기 검토 범위를 같이 남긴다.
