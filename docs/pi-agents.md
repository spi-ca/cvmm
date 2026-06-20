# Pi role agents

이 저장소는 `cvmm` 작업을 위해 repo-local Pi resources를 관리한다. 역할 기반 workflow는 Go 코드, `cloud-hypervisor`/`virtiofsd` orchestration, YAML/OpenAPI manifest, systemd unit, `.pi` asset을 현재 diff와 명령 결과로 검증하도록 설계한다.

## Resource inventory

Repo-local Pi discovery 기준 현재 구성:

- Subagents: `.pi/agents/*.md` 7개
- Skills: `.pi/skills/**/SKILL.md` 4개
- Prompt templates: `.pi/prompts/*.md` 2개
- Extensions/guardrails: `.pi/extensions/*.json` 2개
- Project-local `settings.json`: `.pi/settings.json` 1개 (`theme: "dark"` built-in theme)

Trusted project에서만 repo-controlled Pi resources를 사용한다. 실행 시 확인이 요구되면 우회하지 않는다.

## Subagent roles

| Agent | 권한 | 역할 |
| --- | --- | --- |
| `user-representative` | read-only, no bash | 사용자 의도, acceptance criteria, user-facing scenario, blocker 정리 |
| `software-systems-engineer` | read-only + bash | Linux/runtime/dependency/path/service/operational constraint 검토 |
| `software-designer` | read-only + bash | 설계 결정, 변경 범위, 구현 계획, 검증 전략 작성 |
| `software-developer` | edit-capable + bash | 독립 work package 구현, focused validation, 병렬 lane 안전성 보고 |
| `software-implementer` | edit-capable + bash | 승인된 계획 또는 lane 결과 통합, 파일 수정, focused validation |
| `software-qa` | read-only + bash | acceptance criteria 기반 테스트/스모크/evidence 검증 |
| `software-reviewer` | read-only + bash | diff 품질, regression, completion evidence audit |

현재 `.pi/agents/*.md` frontmatter 확인 기준 상세 surface는 아래와 같다.

| File | model | thinking | tools |
| --- | --- | --- | --- |
| `.pi/agents/user-representative.md` | `openai-codex/gpt-5.4-mini` | `low` | `read`, `find`, `ls` |
| `.pi/agents/software-systems-engineer.md` | `openai-codex/gpt-5.4` | `high` | `read`, `find`, `ls`, `bash` |
| `.pi/agents/software-designer.md` | `openai-codex/gpt-5.4` | `high` | `read`, `find`, `ls`, `bash` |
| `.pi/agents/software-developer.md` | `openai-codex/gpt-5.4` | `high` | `read`, `find`, `ls`, `bash`, `edit`, `write` |
| `.pi/agents/software-implementer.md` | `openai-codex/gpt-5.4` | `high` | `read`, `find`, `ls`, `bash`, `edit`, `write` |
| `.pi/agents/software-qa.md` | `openai-codex/gpt-5.4-mini` | `medium` | `read`, `find`, `ls`, `bash` |
| `.pi/agents/software-reviewer.md` | `openai-codex/gpt-5.4` | `high` | `read`, `find`, `ls`, `bash` |

편집 가능 agent는 `software-developer`, `software-implementer`뿐이고 둘 다 `edit`와 `write`를 함께 가진다. `bash`가 없는 agent는 `user-representative`뿐이다. 병렬 lane 전제는 `software-developer`에만 둔다.

## Skills

- `.pi/skills/software-role-agents/SKILL.md`: 역할 기반 workflow와 repo-specific quality gate
- `.pi/skills/software-developer-parallel/SKILL.md`: 승인된 work를 독립 Go/docs/config package로 나누어 병렬 개발
- `.pi/skills/iterative-findings-loop/SKILL.md`: QA/review finding을 해소할 때까지 구현·검증 반복
- `.pi/skills/run-cvmm-validation/SKILL.md`: cvmm validation/measurement helper

## Prompt templates

- `.pi/prompts/software-role-workflow.md`: 역할 순서 기반 workflow. 시작 부분에 `.pi/prompts/` template이며 `$ARGUMENTS`가 trailing user input이라는 note를 둔다.
- `.pi/prompts/software-developer-parallel.md`: 병렬 developer lane 구성. 시작 부분에 같은 template note를 둔다.

Recommended large-workflow order:

1. 사용자 대표 관점으로 요구사항과 acceptance criteria 정리
2. 시스템 엔지니어 관점으로 binary path, socket, permission, runtime/service 제약 확인
3. 설계자 관점으로 변경 범위와 검증 전략 작성
4. 독립 package가 안전하면 `software-developer` 병렬 lane 사용
5. `software-implementer`가 공유 파일 통합과 잔여 구현 수행
6. `software-qa`와 `software-reviewer`가 current evidence로 검증
7. blocking finding이 있으면 iterative findings loop로 반복

## Preferred validation surfaces

변경된 surface에 맞는 검증을 고른다.

- Go 코드/CLI/config builder: `gofmt -w .`, `go vet ./...`, `go test ./...`
- YAML/OpenAPI manifest: local YAML parser로 touched file parse
- systemd unit (`contrib/cvmm@.service`): `systemd-analyze`와 host-installed `/usr/bin/cvmm` 또는 동등한 unit override path가 있으면 `systemd-analyze verify contrib/cvmm@.service`
- `.pi`/docs-only 변경: `go test ./...`, docs inventory listing, JSON parse, frontmatter spot-check, `git diff --check`
- `cloud-hypervisor`/`virtiofsd` command line, socket path, startup/shutdown flow를 바꾸면 관련 command/config assembly evidence 또는 focused test를 남긴다.

## Guardrails

- `.pi/extensions/guardrails.json`을 현재 기준으로 우선 본다.
- `.pi/extensions/guardrails.v0.json`은 과거 제약 비교가 필요할 때만 읽는다.
- Guardrail 설명을 바꾸면 `.pi/extensions/*.json`의 `$schema`, `version`, `allowedPaths`, current/legacy 역할을 실제 파일과 대조한다.
- 문서/Pi resource 변경도 `AGENTS.md`와 [`operations.md`](operations.md)의 검증 기준을 따른다.
- `.pi/agents/*.md`를 수정하거나 이 문서의 agent/frontmatter summary를 바꿨다면 summary와 실제 `.pi/agents/*.md` frontmatter(model/thinking/tools)를 대조해 검증한다.

## Completion criteria

- 사용자 요구사항의 모든 명시 항목이 파일 내용, command output, diff, test, log, artifact 같은 current evidence에 매핑된다.
- 작은 작업에서 subagent를 생략했다면 root agent가 해당 관점의 판단을 요약한다.
- `software-developer` 병렬 lane을 썼다면 lane별 allowed files, changed files, validation, conflict 여부가 남아 있다.
- `.pi/agents`를 편집했거나 이 문서의 frontmatter summary를 갱신했다면 summary/count가 실제 `.pi/agents/*.md`와 일치한다.
- `settings.json`을 유지한다면 documented value가 실제 파일과 일치한다.
- YAML/OpenAPI/service/Go/Pi resource 변경마다 맞는 repo-native 검증이 실행됐다.
- current implementation evidence와 다른 프로젝트의 historical benchmark contract를 섞지 않는다.
- 미승인 shortcut, 임시 미완성 표식, hidden assumption, duplicated logic, 문서화되지 않은 behavior change를 남기지 않는다.
