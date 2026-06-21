# cvmm benchmarks

현재 저장소에는 `cvmm`용 공식 benchmark harness가 없다. 따라서 이 문서는 이전 레거시 benchmark 계약을 계승하지 않고, 앞으로의 측정 규칙만 정의한다. 구체적인 benchmark 후보와 테스트 갭은 [`test-and-benchmark-gaps.md`](test-and-benchmark-gaps.md)를 함께 본다.

## 원칙

성능 주장에는 아래가 함께 남아야 한다.

- 사용한 binary와 git revision
- host OS / kernel / CPU / storage 요약
- `cloud-hypervisor`, `virtiofsd`, `passt` 버전 또는 binary path
- network backend(`passt`/`tap`)와 helper binary path/version
- node manifest와 image 이름
- 실행한 명령
- raw output 또는 원시 측정값
- 같은 변경 직후의 `go test ./...` 결과

## 우선 측정 대상

- `start NODE` end-to-end latency
- API readiness (`vmm-ping`)까지의 대기 시간
- 기본 `passt` helper spawn + socket readiness 시간
- `vm-info` / `vm-counters` API round-trip latency
- `console NODE` attach latency
- 공유 디렉터리 수 증가에 따른 `virtiofsd` fan-out 비용
- `tap` 호환 경로 대비 `passt` 기본 경로 startup/shutdown 비용 차이
- `shutdown NODE` 종료 시간

## 최소 claim 기준

작은 smoke 한 번으로 성능 향상을 주장하지 않는다. 최소한:

- 같은 머신/같은 manifest/같은 이미지로 before/after 비교
- 반복 측정 여러 회
- 평균만이 아니라 편차나 percentile 기록
- 실패/timeout 케이스도 함께 보관

`passt` 기본 경로와 `tap` 호환 경로는 같은 표에서 섞지 말고 backend를 명시해 분리 기록한다.

## 비목표

- 현재 저장소에 없는 벤치마크 스크립트를 이미 존재하는 것처럼 문서화
- 제거된 legacy non-cvmm artifact archive를 `cvmm` 성능 근거로 재사용
