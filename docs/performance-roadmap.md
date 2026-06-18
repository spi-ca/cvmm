# cvmm performance roadmap

이 문서는 `cvmm`의 성능 후속 과제를 적는다. 현재는 correctness와 문서 정리가 우선이며, 공식 benchmark harness는 아직 없다.

## 우선순위

1. `start NODE` 경로 분해 측정
   - manifest load
   - image path resolution
   - cloud-hypervisor readiness 대기
   - `vm.create` / `vm.boot`
2. `client` API 왕복 시간 측정
   - `vm-info`
   - `vm-counters`
   - bodyful actions
3. `virtiofsd` fan-out 비용 측정
   - shared directory 수 증가
   - restart/reconcile 비용
4. console attach 응답성 확인
5. pid/socket cleanup 실패 시 재시도 비용 관찰

## 가드레일

- 성능 작업이 CLI surface를 바꾸는 근거가 되어서는 안 된다.
- node manifest를 우회하는 hidden config source를 추가하지 않는다.
- pid/socket 파일 정리 의미론을 성능 목적으로 약화하지 않는다.
- legacy `docs/artifacts/**`를 현재 roadmap 근거로 섞지 않는다.

## 완료 기준

성능 개선을 문서화하려면:

- before/after 측정이 있어야 하고
- 같은 조건 비교여야 하며
- `go test ./...`가 함께 통과해야 한다.
