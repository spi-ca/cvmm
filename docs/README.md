# Documentation

이 디렉터리는 `cvmm` maintainer 문서를 모은다. 현재 active 문서는 Go 구현, cloud-hypervisor orchestration, YAML manifest 계약을 설명한다.

> 이전 `docs/artifacts/**` legacy archive는 현재 `cvmm` 기준의 active evidence가 아니어서 제거했다. 새 artifact는 explicit `cvmm` provenance가 있을 때만 추가한다.

## 읽기 순서

1. [`../README.md`](../README.md) - 프로젝트 개요와 CLI
2. [`requirements.md`](requirements.md) - 운영 전제와 manifest 계약
3. [`design.md`](design.md) - 런타임 흐름과 설계 의도
4. [`architecture.md`](architecture.md) - 패키지 구조와 데이터 흐름
5. [`commenting.md`](commenting.md) - Go package 주석 커버리지 기준
6. [`operations.md`](operations.md) - 실행/검증 runbook
7. [`benchmarks.md`](benchmarks.md) - 현재 성능 측정 방침
8. [`test-and-benchmark-gaps.md`](test-and-benchmark-gaps.md) - 가능한 benchmark와 누락된 테스트 보고
9. [`performance-roadmap.md`](performance-roadmap.md) - 성능 후속 과제

## 저장소 맵

```text
cvmm/
├── README.md
├── AGENTS.md
├── CLAUDE.md
├── go.mod
├── main.go
├── internal/
│   ├── entry/   # CLI action entrypoints
│   ├── hvm/     # hypervisor orchestration and API client
│   ├── model/   # manifest and cloud-hypervisor request models
│   └── util/    # pid, PTY, user/process helpers
│       └── sys/ # OS/syscall helpers behind build-specific files
├── contrib/     # deployment and integration examples
└── docs/        # maintainer-facing documentation
```

## 문서별 역할

- `requirements.md` - 무엇을 지원하고 무엇을 지원하지 않는지
- `design.md` - manifest를 VM/virtio-fs runtime으로 변환하는 방식
- `architecture.md` - 패키지별 책임과 lifecycle
- `commenting.md` - package/function/type 주석 커버리지와 audit 기준
- `operations.md` - 실행 방법, 점검 명령, evidence 기준
- `benchmarks.md` - 공식 벤치마크 harness 부재와 측정 규칙
- `test-and-benchmark-gaps.md` - benchmark 후보와 테스트 갭 우선순위
- `performance-roadmap.md` - 우선순위 backlog
