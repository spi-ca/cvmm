# Documentation

이 디렉터리는 `cvmm` maintainer 문서를 모은다. 현재 active 문서는 Go 구현, cloud-hypervisor orchestration, YAML manifest 계약, 기본 `passt` + explicit TAP compatibility network 동작을 설명한다.

> 이전 `docs/artifacts/**` legacy archive는 현재 `cvmm` 기준의 active evidence가 아니어서 제거했다. 새 artifact는 explicit `cvmm` provenance가 있을 때만 추가한다.

## 읽기 순서

1. [`../README.md`](../README.md) - 프로젝트 목적, 쓰임새, 기본 사용법
2. [`requirements.md`](requirements.md) - 지원 범위와 manifest 계약
3. [`design.md`](design.md) - manifest를 VM/virtio-fs/runtime helper로 변환하는 방식
4. [`architecture.md`](architecture.md) - package 책임과 lifecycle
5. [`operations.md`](operations.md) - 실행 방법, 점검 명령, evidence 기준
6. [`migration-adr-001-passt-network.md`](migration-adr-001-passt-network.md) - ADR-001 network manifest migration checklist
7. [`adr/0001-passt-default-network-backend.md`](adr/0001-passt-default-network-backend.md) - default `passt` 결정 배경, migration 의도, 구현 시 기대한 검증 기준
8. [`adr/0002-adaptive-thp-handling.md`](adr/0002-adaptive-thp-handling.md) - guest THP adaptive handling 결정, 구현 범위, fallback 기록
9. [`diagrams/README.md`](diagrams/README.md) - diagram source와 SVG/PNG 산출물 규칙
10. [`adr/`](adr/) - 구현된 결정과 future-facing decision record를 함께 포함

## 필요할 때만 열 문서

- [`commenting.md`](commenting.md) - Go package/function/type 주석 audit 기준
- [`correctness-guardrail-evidence.md`](correctness-guardrail-evidence.md) - 문서/코드 correctness guardrail과 현재 evidence
- [`migration-adr-001-passt-network.md`](migration-adr-001-passt-network.md) - legacy `net_*` manifest를 nested `net`/default `passt` 또는 explicit `tap`으로 옮기는 방법
- [`benchmarks.md`](benchmarks.md) - 공식 benchmark harness 부재와 측정 규칙
- [`test-and-benchmark-gaps.md`](test-and-benchmark-gaps.md) - benchmark 후보와 남은 테스트/운영 evidence gap 우선순위
- [`performance-roadmap.md`](performance-roadmap.md) - 성능 후속 backlog
- [`pi-agents.md`](pi-agents.md) - repo-local Pi agents/skills/prompt resources
- [`adr/0001-passt-default-network-backend.md`](adr/0001-passt-default-network-backend.md) - 현재 기본 `passt` backend의 결정 배경과 migration note. 구현 세부 동작은 `design.md`/`operations.md`/`architecture.md`를 우선 본다.
- [`adr/0002-adaptive-thp-handling.md`](adr/0002-adaptive-thp-handling.md) - guest THP를 host kernel 지원 여부에 맞춰 adaptive하게 다루는 결정 기록. 현재 구현은 start-time host probe, explicit `thp:false`, THP 관련 `vm.create` 1회 retry, default shared-memory path의 `mergeable=on` 생략을 포함한다.
- [`guidelines/`](guidelines/) - agent-facing documentation guidelines; repository rule source가 아니라 정리 기준 참고 자료
