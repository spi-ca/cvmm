# ADR 0002: Make guest THP handling adaptive to host kernel support

- Status: Implemented
- Date: 2026-06-22

> Implementation note: `cvmm start` now leaves manifest-derived memory THP unset until runtime, probes Linux THP/shmem policy plus `PR_GET_THP_DISABLE`, emits explicit `thp: false`/`thp=off` when host support is unavailable, and retries one THP-related `vm.create` failure with THP disabled before boot.

## Context

이 ADR 작성 당시 `cvmm`의 guest memory 설정은 THP를 항상 요청했다.

- `internal/model/config.go`의 `Config.MemoryConfig()`는 `Shared: true`와 `Thp: true`를 고정했다.
- `internal/model/api.go`의 `MemoryConfig.CommandArgs()`는 `Thp`가 `true`일 때만 `thp=on`을 렌더링했고, 당시 모델에는 명시적 disabled 상태가 없었다.
- `contrib/cloud-hypervisor.yaml`의 `MemoryConfig.thp` 스키마 기본값은 `true`다.

이 조합 때문에 구현은 `thp=on`을 단순 생략하는 것만으로 THP를 확실히 끄는 방식에 의존할 수 없었다. upstream 기본값이 이미 `true`이기 때문이다.

현재 `cvmm`는 guest memory를 계속 `shared=on`으로 사용한다. 따라서 THP 판단은 일반 THP 가능 여부만이 아니라 shared-memory-backed guest RAM에 영향을 주는 host의 shmem THP 정책까지 함께 고려해야 한다. 이 부분은 Linux/cloud-hypervisor 동작 검토를 기반으로 한 설계 입력이며, 현재 저장소에는 관련 unit test도 포함된다.

구현 후 `contrib/minimal` fixture를 cloud-hypervisor v52.0으로 기동해 본 로컬 E2E에서는 adaptive THP가 `thp: false`를 명시하는 데까지는 동작했지만, 당시 기본 payload가 `mergeable=on,shared=on`을 함께 보내 `vm.create` validation이 아래 error로 실패했다.

```text
Invalid to set both 'mergeable' and 'shared' for memory
```

이 에러는 THP 자체가 아니라 cloud-hypervisor v52의 shared memory와 KSM mergeable memory 조합 제약이다. upstream v52 source/release-note 검토 결과, `shared=true`와 `mergeable=true`는 함께 유효하지 않은 조합으로 다뤄진다. 현재 구현은 `shared=on` 경로에서 default `mergeable=on`을 생략해 이 조합을 피한다. explicit API/client payload가 `mergeable: true`를 지정하는 표현 능력은 유지하지만, manifest-derived 기본 shared-memory payload는 KSM mergeable을 켜지 않는다.

설계 검토의 출발점은 아래 사용자 요구사항이었다.

- THP를 지원하지 않는 커널에서는 THP를 적용하지 않는다.
- THP를 지원하는 커널에서는 THP를 적용한다.
- disabled 결과는 생략이 아니라 명시적 off로 전달한다.

## Decision / implemented target

구현은 아래 목표를 따른다.

1. THP 판단은 manifest가 아니라 host runtime capability를 기준으로 자동 결정한다.
2. THP 미지원 또는 비활성 host에서는 `start`가 THP 때문에 실패하지 않아야 한다.
3. THP 지원 host에서는 기존 shared guest memory 구성을 유지하면서 THP를 요청한다.
4. THP 미지원 host에서 THP를 끄는 결과는 cloud-hypervisor request 모델에서 명시적으로 표현되어야 한다. 단순히 `thp` 필드를 생략하는 방식에는 의존하지 않는다.
5. `cvmm`의 `vm.create`는 JSON request body를 사용하므로, disabled 상태는 CLI 문자열뿐 아니라 JSON payload에서도 `thp: false`가 실제 wire에 실리도록 표현되어야 한다.
6. 첫 구현 범위에는 새 manifest field, CLI flag, environment variable을 추가하지 않는다. operator override가 필요하면 별도 ADR로 다룬다.
7. shared memory를 쓰는 `cvmm` 기본 경로는 cloud-hypervisor v52에서 `mergeable=on`을 함께 보내지 않는다. 이 제약은 THP on/off 판단과 별개이며, `shared=on` + adaptive `thp`는 유지하고 KSM용 default `mergeable`만 생략한다.

## Implementation options reviewed

### Option A. Keep unconditional `thp=on`

장점:

- 현재 코드와 가장 가깝다.
- 추가 모델 변경이 없다.

단점:

- THP 미지원 커널에서도 항상 THP를 요청한다.
- 사용자 요구사항을 충족하지 못한다.
- shared memory 경로에서 host 정책 차이를 문서화해도 런타임 적응이 없다.

### Option B. On unsupported kernels, omit `thp=on`

장점:

- 구현이 단순해 보인다.
- 현재 bool 모델을 덜 건드릴 수 있다.

단점:

- `contrib/cloud-hypervisor.yaml` 기준 upstream 기본값이 `true`이므로, 단순 생략은 비활성화를 보장하지 못한다.
- "지원되지 않으면 적용하지 않는다"는 목표를 만족시키지 못할 수 있다.

이 옵션은 선택하지 않는다.

### Option C. Add a manifest/CLI/env knob now

장점:

- operator가 즉시 강제 제어할 수 있다.
- host probing 실패 시 우회 수단이 생긴다.

단점:

- 현재 요구사항은 자동 적응이며, 사용자 입력 surface 확장은 우선순위가 아니다.
- manifest/CLI/env 계약을 늘리면 문서, validation, migration 범위가 불필요하게 커진다.
- 기본 자동 정책이 정리되기 전에 override를 먼저 노출하면 의미가 불명확해진다.

이 옵션은 future consideration으로 남긴다.

### Option D. Add host-side THP detection plus an internal explicit on/off state

개요:

- `start` 경로에서 host THP/shmem THP 지원 여부를 판단한다.
- `internal/model`의 memory 표현은 나중에 tri-state 또는 이에 준하는 명시적 on/off 상태를 표현할 수 있어야 한다.
- 지원되면 THP enabled request를 만들고, 지원되지 않으면 explicit disabled 결과를 만든다.
- host가 THP를 완전히 지원하지 않거나 `madvise` 계열 최적화가 실패해도 VM startup 전체는 non-fatal로 유지한다.
- host probe가 불완전하거나 틀려서 THP-enabled `vm.create`가 THP 관련 이유로 실패하는 경우, boot 전 한 번 explicit disabled request로 강등 재시도를 고려한다.

장점:

- 사용자 요구사항에 가장 직접적으로 맞는다.
- runtime host 차이를 manifest schema에 새로 새기지 않아도 된다.
- 단순 생략이 아닌 explicit disabled semantics를 설계에 포함할 수 있다.

단점:

- 현재 bool 기반 API 모델 변경이 필요하다.
- host detection과 logging 규칙을 새로 검증해야 한다.
- sysfs/배포 환경 차이에 대한 테스트 fixture가 필요하다.

## Selected approach

이 ADR은 **Option D**를 선택한다.

구현 시 방향은 아래와 같다.

1. THP capability 판단은 `internal/hvm`의 runtime start path에 둔다. host kernel과 sysfs 상태는 manifest 속성이 아니라 배포 환경 속성이기 때문이다.
2. 그 전에 `internal/model`의 memory 표현은 explicit enabled/disabled를 담을 수 있게 바꿔야 한다. bool 하나로는 현재 목표를 표현하기 어렵고, `omitempty` bool은 JSON `vm.create` 본문에 `thp: false`를 보낼 수 없다.
3. shared guest memory를 유지하므로, host 점검은 일반 THP enablement와 shared-memory 관련 THP 정책을 함께 다뤄야 한다.
4. THP 관련 host 한계는 가능하면 warning/decision log로 남기고, VM startup의 hard failure 조건으로 승격하지 않는다.
5. host probe 결과가 THP 지원으로 나왔더라도 `vm.create`가 THP 관련 validation/error로 실패하면, `vm.boot` 전 상태에서 explicit disabled request로 한 번 재시도하는 fallback을 설계한다. 에러 원인 식별이 불가능하면 무조건 재시도하지 말고 기존 실패 경로와 cleanup을 유지한다.
6. 최종 운영 로그와 검증 evidence는 pre-probe `Load` 시점 config가 아니라 THP decision이 반영된 최종 `vm.create` payload를 기준으로 남겨야 한다. 필요하면 `hypervisor config` 로그 위치나 추가 decision log를 조정한다.
7. 구현 첫 단계에서는 user-facing knob를 추가하지 않는다.

이 ADR은 정확한 sysfs probing 순서나 logging 문구까지 고정하지 않는다. 다만 "지원되지 않으면 명시적으로 끈 상태를 전달해야 한다"는 제약은 고정한다.

## Shared memory and mergeable

`mergeable`은 KSM 관련 설정이고, `thp`는 transparent huge page 요청이다. 둘은 같은 optimization 계층이 아니다. `cvmm`가 virtio-fs/vhost-user 계열을 위해 `shared=on`을 유지하는 한, cloud-hypervisor v52 호환성을 위해 기본 memory payload에서 `mergeable=on`을 제거하는 것이 적절하다.

구현 방향은 아래와 같다.

1. `internal/model/config.go`의 기본 `MemoryConfig()`에서 `Shared: true`는 유지하되 `Mergeable: true`를 설정하지 않는다.
2. `internal/model/api.go`의 일반 렌더링 기능은 explicit `Mergeable: true`가 주어진 API/client payload를 계속 표현할 수 있어야 한다. 즉, `mergeable` 필드를 전역 삭제하지 않는다.
3. 기본 VM config, model golden, minimal E2E 기대값에서 `--memory ... mergeable=on,shared=on` 조합을 제거한다.
4. 이 변경은 새 manifest field, CLI flag, env knob를 추가하지 않는다.
5. `shared=false`인 future memory mode가 도입되면 그때 KSM mergeable default를 별도 설계한다.

## Non-goals

이 ADR은 아래를 지금 결정하지 않는다.

- manifest에 `memory.thp` 같은 새 field 추가
- `--thp-*` 류 CLI flag 또는 environment variable 추가
- hugepages 설정과 THP 정책을 하나의 knob로 통합
- NUMA, memory hotplug, hugepage pool provisioning 재설계
- 실제 cloud-hypervisor upstream default를 변경하거나 patch하는 일
- explicit API/client payload에서 `mergeable: true` 표현 능력을 제거하는 일

## Validation evidence

현재 구현은 최소한 아래 증거를 기준으로 유지한다.

- 자동 테스트:
  - `internal/model` 테스트가 THP enabled/disabled/unset 표현이 cloud-hypervisor JSON payload와 CLI args에 어떻게 렌더링되는지 검증한다. 특히 disabled 상태는 `vm.create` JSON body에 `thp: false`가 포함되어야 한다.
  - 기본 shared-memory path가 `mergeable=on`을 생략하고, explicit `Mergeable: true` 렌더링 기능은 유지되는지 검증한다.
  - host detection 테스트가 sysfs fixture 또는 helper abstraction으로 supported/unsupported/shmem-disabled 조합을 검증한다.
  - start-path 테스트가 지원 host에서는 enabled request를, 미지원 host에서는 explicit disabled request를 선택하는지 검증한다.
  - probe-miss/validation-error 상황은 THP 관련 `vm.create` 실패 후 explicit disabled 재시도를 검증한다.
  - client wire-body 테스트가 직접 `vm.create`와 stdin YAML client 경로 모두에서 `thp:false` 직렬화를 검증한다.
- 운영/통합 evidence:
  - THP 지원 host와 비지원 또는 비활성 host 각각에서 실제 `start` 결과와 최종 `vm.create` memory JSON/decision log를 수집해야 한다.
  - THP warning 또는 fallback이 발생해도 기존 shutdown/cleanup 경로가 유지되는지 실제 runtime evidence를 수집해야 한다.
  - cloud-hypervisor v52 계열에서는 `contrib/minimal` 기반 E2E가 `shared=on` 기본 경로를 `mergeable=on` 없이 `Running`까지 올리는지 확인한다. 현재 로컬 evidence는 `docs/correctness-guardrail-evidence.md`에 기록되어 있다.
- 문서 검증: `README.md`, `docs/README.md`, 관련 설계/운영 문서가 구현 상태와 남은 privileged evidence gap을 구분하는지 확인한다.

## Rollout risks

- explicit disabled semantics 없이 구현을 시작하면 upstream default `true` 때문에 실제로는 THP가 계속 켜질 수 있다.
- sysfs 기반 host 판단은 container, custom kernel, restricted mount 환경에서 정보가 불완전할 수 있다.
- shared guest memory와 shmem THP 정책이 어긋나면 일반 THP만 확인하고 잘못 판단할 수 있다.
- cloud-hypervisor가 THP 관련 최적화를 warning으로만 처리하는 경우, 로그를 놓치면 운영자가 "적용되었다"고 오해할 수 있다.
- user override를 일부러 미루므로, 초기 구현은 자동 판단이 틀릴 때 즉시 수동 교정 수단이 없다.
- `shared=on`과 `mergeable=on`을 같이 유지하면 cloud-hypervisor v52에서 THP decision 이전/이후와 무관하게 boot validation이 실패할 수 있다.

## Consequences

- 구현 단계에서는 `internal/model/config.go`, `internal/model/api.go`, 관련 테스트, start-time host probing, 운영 문서를 함께 조정해야 한다.
- 현재 문서는 start-time host probing, explicit disabled semantics, THP-related `vm.create` 1회 재시도 범위를 코드와 함께 유지해야 한다.
- `shared=on` 경로에서 `mergeable=on` 생략을 구현하면 이 ADR의 follow-up design note, model tests, operations evidence를 함께 현재 구현 상태로 갱신해야 한다.
- 별도 operator override가 필요하다고 판단되면 후속 ADR이 필요하다.
