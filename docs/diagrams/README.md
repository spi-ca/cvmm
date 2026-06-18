# Mermaid diagram artifacts

`docs/diagrams`는 현재 테스트 수치가 없는 상태에서 시스템 구조와 호출 흐름을 설명하기 위한 Mermaid source of truth다. 향후 측정값을 제시할 때는 이 다이어그램들이 어떤 component, sequence, host operation이 측정 대상인지 설명하는 배경 자료가 된다.

- 원본: `*.mmd`
- 공용 Mermaid 설정: `mermaid-config.json`
- 공용 Puppeteer 설정: `puppeteer-config.json`
- 산출물: 같은 basename의 `*.svg`, `*.png`

## Config 역할

- `mermaid-config.json`: theme, font, color 같은 Mermaid 렌더링 스타일을 정의한다.
- `puppeteer-config.json`: Mermaid CLI가 Chromium/Puppeteer를 띄울 때 사용할 launch args를 정의한다.
- 주의: 이 두 JSON만으로는 PNG 2x scale이 자동 보장되지 않는다. PNG 해상도 규칙은 렌더 명령에서 Mermaid CLI `--scale 2`로 강제한다.

## 다이어그램 목록

- `system-context.mmd`: component diagram. caller, `cvmm`, host storage, runtime process, guest boundary를 한눈에 보여준다.
- `request-decision-flow.mmd`: sequence diagram. `cvmm start NODE_NAME`의 manifest load, API readiness, `VmCreate`, `virtiofsd` reconcile, boot, shutdown path를 보여준다.
- `path-resolution.mmd`: runtime path and host-operation diagram. node/image root, `run/` 파일, socket, 대표 host operation(`open`, `stat`, `exec`, Unix socket HTTP, pid file, signal)을 연결한다.
- `module-architecture.mmd`: Go package/component dependency diagram. `main.go`, `internal/entry`, `internal/hvm`, `internal/model`, `internal/util`의 책임 경계를 보여준다.
- `visibility-axis.mmd`: manifest mapping diagram. `image`, `disk[]`, `directory[]`, network/default fields가 cloud-hypervisor payload와 `virtiofsd` config로 바뀌는 흐름을 보여준다.
- `mutability-axis.mmd`: validation/evidence flow diagram. 문서, Go 코드, 다이어그램, container/deploy, runtime measurement 변경별 검증 증거 흐름을 보여준다.

## 저장소 기준 렌더링 규칙

- SVG는 위 두 config를 함께 사용해 렌더링한다.
- PNG는 같은 config를 사용하되 반드시 `--scale 2`를 추가한다.
- 새 다이어그램을 추가하거나 `*.mmd`, `mermaid-config.json`, `puppeteer-config.json`을 바꾸면 대응 `*.svg`, `*.png`를 같이 재생성한다.

## 예시 명령

단일 SVG:

```bash
bunx @mermaid-js/mermaid-cli \
  -i docs/diagrams/system-context.mmd \
  -o docs/diagrams/system-context.svg \
  -c docs/diagrams/mermaid-config.json \
  -p docs/diagrams/puppeteer-config.json
```

단일 PNG(2x 필수):

```bash
bunx @mermaid-js/mermaid-cli \
  -i docs/diagrams/system-context.mmd \
  -o docs/diagrams/system-context.png \
  -c docs/diagrams/mermaid-config.json \
  -p docs/diagrams/puppeteer-config.json \
  --scale 2
```

전체 다이어그램 재생성:

```bash
for input in docs/diagrams/*.mmd; do
  base="${input%.mmd}"

  bunx @mermaid-js/mermaid-cli \
    -i "$input" \
    -o "${base}.svg" \
    -c docs/diagrams/mermaid-config.json \
    -p docs/diagrams/puppeteer-config.json

  bunx @mermaid-js/mermaid-cli \
    -i "$input" \
    -o "${base}.png" \
    -c docs/diagrams/mermaid-config.json \
    -p docs/diagrams/puppeteer-config.json \
    --scale 2

done
```

## 확인

재생성 후에는 적어도 다음을 확인한다.

```bash
file docs/diagrams/*.png
grep -R "old-project-name-pattern" -n docs/diagrams/*.mmd docs/diagrams/README.md
```

실제 점검에서는 이전 프로젝트명, 이전 언어/runtime명, 이전 filesystem 용어가 현재 다이어그램에 남지 않았는지 확인한다. 그리고 문서 diff에서 `docs/architecture.md`, `docs/design.md`, `README.md`가 같은 시스템 경계를 가리키는지 검토한다.
