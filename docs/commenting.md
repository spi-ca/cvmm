# 주석 및 문서화 기준

`cvmm`의 Go module `amuz.es/src/spi-ca/cvmm`는 아래 package/component를 active 구현 범위로 본다.

- `main`
- `internal/entry`
- `internal/hvm`
- `internal/model`
- `internal/util`
- `internal/util/sys`

## 커버리지 기준

루트 module 개요는 `README.md`가 맡고, 각 package는 package comment로 현재 책임과 경계를 설명한다. 그 위에서 아래 항목은 기본적으로 주석 대상이다.

- 모든 non-test top-level function
- 모든 non-test method
- 모든 struct type
- 중요한 exported contract/value type (`Config`, API request/response, enum-like 값, CLI 입력 계약 등)

주석은 구현을 다시 읽지 않고도 현재 동작, 입력/출력, 중요한 side effect를 파악할 수 있게 쓴다. 테스트 전용 심볼과 자명한 private 상수까지 기계적으로 늘릴 필요는 없지만, exported 계약이나 운영 동작을 오해하게 만드는 공백은 남기지 않는다.

## 작성 규칙

- 현재 동작만 설명한다.
- TODO, 추측, 미래 계획을 계약처럼 쓰지 않는다.
- manifest/runtime/socket/path 같은 용어는 현재 저장소 문서와 같은 이름을 쓴다.
- build tag 파일에서는 `//go:build`/`// +build`와 doc comment의 위치를 섞지 않는다.
- cgo 파일에서는 `import "C"` 바로 앞 special comment와 Go doc comment가 충돌하지 않게 분리한다.

## 감사 방식

리뷰나 정리 작업에서는 package 단위로 누락을 기록하고, 가능하면 parser 기반 점검 결과를 함께 남긴다. parser audit은 임시 파일에서 `go/parser`/`go/doc`를 사용해 실행하거나 이후 저장소 스크립트로 고정해도 된다.

예시 검증 명령:

```bash
go test ./...
go vet ./...
{ printf '%s\n' README.md AGENTS.md CLAUDE.md; find docs -maxdepth 2 -type f; } | sort
```

코드와 주석을 함께 손댄 변경에서는 추가로 아래를 포함한다.

```bash
gofmt -w .
```
