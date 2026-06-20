# Documentation

이 디렉터리는 `cvmm` maintainer 문서를 모은다. 현재 active 문서는 Go 구현, cloud-hypervisor orchestration, YAML manifest 계약을 설명한다.

> 이전 `docs/artifacts/**` legacy archive는 현재 `cvmm` 기준의 active evidence가 아니어서 제거했다. 새 artifact는 explicit `cvmm` provenance가 있을 때만 추가한다.

## 읽기 순서

1. [`../README.md`](../README.md) - 프로젝트 목적, 쓰임새, 기본 사용법
2. [`requirements.md`](requirements.md) - 지원 범위와 manifest 계약
3. [`design.md`](design.md) - manifest를 VM/virtio-fs runtime으로 변환하는 방식
4. [`architecture.md`](architecture.md) - package 책임과 lifecycle
5. [`operations.md`](operations.md) - 실행 방법, 점검 명령, evidence 기준
6. [`diagrams/README.md`](diagrams/README.md) - diagram source와 SVG/PNG 산출물 규칙

## 필요할 때만 열 문서

- [`commenting.md`](commenting.md) - Go package/function/type 주석 audit 기준
- [`correctness-guardrail-evidence.md`](correctness-guardrail-evidence.md) - 문서/코드 correctness guardrail과 현재 evidence
- [`benchmarks.md`](benchmarks.md) - 공식 benchmark harness 부재와 측정 규칙
- [`test-and-benchmark-gaps.md`](test-and-benchmark-gaps.md) - benchmark 후보와 테스트 gap 우선순위
- [`performance-roadmap.md`](performance-roadmap.md) - 성능 후속 backlog
- [`pi-agents.md`](pi-agents.md) - repo-local Pi agents/skills/prompt resources
- [`guidelines/`](guidelines/) - agent-facing documentation guidelines; repository rule source가 아니라 정리 기준 참고 자료
