---
name: software-role-agents
description: Run a software-writing workflow with Pi subagents for base required roles plus a conditional systems engineer and parallel-capable developer.
---

# Software Role Agents

Use this skill to run larger cvmm tasks through the project-local role agents.

Base required roles for every task: `user-representative`, `software-designer`, `software-implementer`, `software-qa`, and `software-reviewer`.

Conditional roles:
- Use `software-systems-engineer` when OS/runtime/dependency/permission/deployment/operational constraints materially affect the task.
- Use `software-developer` only when the design identifies at least one independent work package.

## When to Use

- New features, bug fixes, refactors, or documentation changes need explicit requirements, implementation, QA, and review evidence.
- The task touches Go CLI code, cloud-hypervisor or virtiofsd orchestration, YAML/OpenAPI manifests, systemd unit files, or repo-local Pi resources.
- You want isolated `subagent` contexts for requirements, conditional system constraints, design, implementation, QA, and review.

## Installed pi-subagent Frontmatter Policy

Use the current `pi-subagent` frontmatter fields only:

- `name`
- `description`
- `model`
- `thinking`
- `tools`

The project role agents use Codex model IDs and a separate `thinking` field.

## Role Responsibilities

### user-representative

- Preserve the user request, success criteria, user-facing scenarios, and blockers.
- Call out missing decisions before implementation starts.

### software-systems-engineer

- Use this role only when OS/runtime/dependency/permission/deployment/operational constraints materially affect the task.
- Inspect Linux runtime assumptions, file paths, sockets, permissions, external binaries, service lifecycle, and operational constraints.
- Pay extra attention when the task touches `cloud-hypervisor`, `virtiofsd`, manifests under `contrib/`, start/stop flows, or systemd units.

### software-designer

- Turn the requirements and any applicable system constraints into an implementation plan.
- Separate Go code, manifests, service files, docs, and Pi asset work into clear packages when possible.
- Define the validation strategy that matches the touched surfaces.

### software-developer

- Implement exactly one approved independent work package.
- Respect the assigned file boundaries, preserved behavior, and focused validation.
- Stop and report blockers if shared-file edits or new cross-package decisions appear.

### software-implementer

- Apply the approved plan or merge approved developer-lane results.
- Keep existing behavior outside the approved scope.
- Finish any shared-file integration and run the required focused validation.

### software-qa

- Map each acceptance criterion to concrete evidence.
- Validate happy path, edge cases, and failure handling when applicable.
- Distinguish blocking findings from non-blocking observations.

### software-reviewer

- Review correctness, maintainability, regression risk, documentation consistency, and completion evidence.
- Check that the final report matches the actual diff and command results.

## Role Input/Output Contract

| Role | Main input | Main output |
| --- | --- | --- |
| `user-representative` | User request, current docs, current behavior evidence | intent summary, acceptance criteria, user-facing scenarios, blockers |
| `software-systems-engineer` (conditional) | requirements output, repo/environment evidence | system constraints, risks, feasibility, system-level validation |
| `software-designer` | requirements + optional system constraints + code/doc structure | change plan, work packages, risks, verification strategy |
| `software-developer` | one approved package with allowed files and acceptance criteria | package diff, focused validation, conflict report, blockers |
| `software-implementer` | approved plan, developer-lane outputs, current files | integrated changes, validation results, blockers |
| `software-qa` | acceptance criteria, diffs, validation surface | QA matrix, command/artifact evidence, findings |
| `software-reviewer` | requirements, design, diff, QA results | review verdict, blocking issues, completion audit |

## Recommended Subagent Calls

Use these supported top-level `subagent` call shapes for sufficiently large tasks. Keep the planning stages in a serial `chain`, run `software-developer` lanes in a separate top-level parallel call only when the designer has separated non-overlapping packages, then run a serial implementation merge call followed by a top-level parallel QA/review call.

### Serial planning chain when system constraints matter

```json
{
  "chain": [
    {
      "agent": "user-representative",
      "task": "Summarize user intent, acceptance criteria, user-facing scenarios, constraints, and blockers for: <task>"
    },
    {
      "agent": "software-systems-engineer",
      "task": "Using the requirements output, inspect repository and environment constraints and provide system-level feasibility, risks, and verification for: <task>"
    },
    {
      "agent": "software-designer",
      "task": "Using requirements and system constraints, design the implementation plan and verification strategy for: <task>"
    }
  ],
  "mode": "spawn"
}
```

### Serial planning chain when system constraints do not matter

```json
{
  "chain": [
    {
      "agent": "user-representative",
      "task": "Summarize user intent, acceptance criteria, user-facing scenarios, constraints, and blockers for: <task>"
    },
    {
      "agent": "software-designer",
      "task": "Using the requirements output directly, design the implementation plan and verification strategy for: <task>. Also record why software-systems-engineer was not needed."
    }
  ],
  "mode": "spawn"
}
```

### Parallel development call

```json
{
  "tasks": [
    {
      "agent": "software-developer",
      "task": "Implement independent work package A for: <task>. Include allowed files, acceptance criteria, preserved behavior, and focused validation."
    },
    {
      "agent": "software-developer",
      "task": "Implement independent work package B for: <task>. Include allowed files, acceptance criteria, preserved behavior, and focused validation."
    }
  ],
  "mode": "spawn"
}
```

If there is exactly one independent work package, use a single top-level call instead of a parallel batch:

```json
{
  "agent": "software-developer",
  "task": "Implement the one approved independent work package for: <task>. Include allowed files, acceptance criteria, preserved behavior, and focused validation.",
  "mode": "spawn"
}
```

### Serial implementation merge call

```json
{
  "agent": "software-implementer",
  "task": "Integrate developer lane results, resolve approved shared-file work, and run focused validation for: <task>",
  "mode": "spawn"
}
```

### Parallel QA and review call

```json
{
  "tasks": [
    {
      "agent": "software-qa",
      "task": "Validate the implementation against acceptance criteria for: <task>"
    },
    {
      "agent": "software-reviewer",
      "task": "Review the implementation, evidence, maintainability, and completion readiness for: <task>"
    }
  ],
  "mode": "spawn"
}
```

## Repo-Specific Validation Surfaces

Pick only the checks that match the changed files:

- Go source, CLI flags, config builders, or internal packages: keep changed Go files formatted and run `go vet ./...` and `go test ./...`.
- YAML or OpenAPI files: parse the touched files with a local YAML parser, for example `python3` + `yaml.safe_load`, or another repo-approved parser.
- systemd unit changes: run `systemd-analyze verify contrib/cvmm@.service` when `systemd-analyze` and host-installed `/usr/bin/cvmm` or an equivalent unit override path are available; otherwise report the missing prerequisite clearly.
- `.pi` or docs-only changes: run `go test ./...`, parse JSON files, spot-check frontmatter, run inventory listings, and run `git diff --check`.
- Changes to cloud-hypervisor or virtiofsd command construction, socket paths, manifests, or start/stop flows should include focused evidence from tests or inspected command/config outputs.

## Single-Agent Fallback

If `subagent` execution is unavailable or the task is small, the root agent should still cover the same responsibilities in order:

1. Requirements
2. System constraints when OS/runtime/dependency/permission/deployment/operational constraints materially affect the task
3. Design
4. Developer package work when applicable
5. Implementation/integration
6. QA
7. Review

## Quality Gates

Before finishing, confirm all of the following:

- Every base required role produced output, or the root agent explicitly covered that role in a small-task fallback.
- If `software-systems-engineer` was skipped, the workflow recorded why the task did not materially depend on OS/runtime/dependency/permission/deployment/operational constraints.
- If `software-developer` lanes were used, each lane reported allowed files, changed files, validation, and any conflict status.
- Every explicit requirement is mapped to current evidence from files, diffs, commands, tests, logs, or artifacts.
- The validation surface matches the touched files instead of using unrelated checks.
- Go changes keep related defaults and wiring aligned across CLI flags, config loading/building, and any touched docs.
- YAML/OpenAPI or systemd-service edits were parsed or verified with an appropriate local command.
- Changes affecting cloud-hypervisor or virtiofsd startup/shutdown behavior include focused evidence for the resulting command/config flow.
- Existing user changes and behavior outside the approved scope were preserved.
- No unapproved shortcuts, placeholder text, dead code, duplicated logic, hidden assumptions, or undocumented behavior changes remain.
- QA and review do not leave unresolved blocking findings.

## Output Template

```markdown
## Role Results

### user-representative
- Goal:
- Acceptance criteria:
- Blockers:

### software-systems-engineer (when used)
- Constraints:
- Risks:
- System validation:

### software-designer
- Design decisions:
- Change scope:
- Verification strategy:

### software-developer
- Work packages:
- Parallel safety:
- Changed files:

### software-implementer
- Changed files:
- Key changes:
- Integration result:

### software-qa
- Commands run:
- Results:

### software-reviewer
- Audit:
- Remaining issues:
- Ready to complete:
```

## Pitfalls

- Do not skip base required roles just because the task feels familiar.
- Do not skip `software-systems-engineer` when the task materially depends on OS/runtime/dependency/permission/deployment/operational constraints.
- Do not parallelize packages that share files, sockets, service definitions, or unresolved design decisions.
- Do not use unrelated legacy filesystem semantics as quality gates for this repo.
- Do not report unrun validations as passing.
- Do not stop at a plan if the request requires implementation and current evidence.
