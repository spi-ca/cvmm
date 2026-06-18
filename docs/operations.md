# 운영 및 검증

## 1. 전제 조건

운영 환경은 아래를 만족해야 한다.

- `cloud-hypervisor` 설치
- `virtiofsd` 설치
- image root와 node root 접근 가능
- pid/socket/runtime 파일을 생성할 권한
- tap/network/capability 설정이 필요한 경우 그에 맞는 실행 권한

예시 systemd unit은 [`../contrib/cvmm@.service`](../contrib/cvmm@.service)를 따른다. 현재 `--runas hvm`는 `cloud-hypervisor` 자식 프로세스 credential에 적용된다. `virtiofsd` helper는 별도 credential 전환 없이 서비스 계정과 그 계정의 capability로 실행되며, `--runas` 사용자의 primary group이 `virtiofsd --socket-group`으로 전달될 수 있다. manifest `directory`는 절대경로도 허용되고 `virtiofsd`는 `--announce-submounts`를 사용하므로, 공유 디렉터리 권한은 서비스 계정/capability와 submount 노출까지, socket 접근 권한은 socket group까지 포함해 설계해야 한다. 배포 unit은 실제 `--node-root` mount dependency와 `User=`/`Group=`/capability bounding을 환경에 맞게 명시해야 한다.

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
  --runas hvm \
  start NODE_NAME
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

- `<node-root>/<node>/config.yaml`이 실제 source of truth인지 확인
- `<node-root>/<node>/run/`의 pid/socket 파일 충돌 여부 확인
- `directory` 항목 수와 `virtiofsd` 프로세스 수가 일치하는지 확인
- `image`가 실제 이미지 디렉터리를 가리키는지 확인
- console 문제 시 `vm-info` 응답의 PTY 경로를 먼저 확인

## 5. Evidence 규칙

- 현재 저장소에는 `cvmm`용으로 정리된 formal benchmark/evidence bundle이 없다.
- 이전 legacy ScreenFS artifact archive는 제거했으며 현재 `cvmm` 운영 근거가 아니다.
- 새 운영 증거를 남길 때는 사용한 manifest, 이미지, 명령, host 환경, stdout/stderr, 검증 시각을 같이 기록한다.
- 주석 audit evidence에는 대상 package, 누락/보강 항목, 사용한 parser 기반 점검 명령 또는 수기 검토 범위를 같이 남긴다.
