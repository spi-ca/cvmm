---
description: Run software work through base role agents with a conditional systems engineer and parallel-capable developer
argument-hint: "<task>"
---
Reminder: This file is a `.pi/prompts/` prompt template. `$ARGUMENTS` is the remaining user input after the prompt command or skill invocation.

Run this software-writing task through the project-local role agents, always using the base required roles and adding conditional roles only when justified:

Task:
$ARGUMENTS

Use the `subagent` tool with supported top-level object shapes only: a serial `subagent({ chain, mode })` call, a parallel `subagent({ tasks, mode })` call, or a single `subagent({ agent, task, mode })` call. Use `mode: "spawn"` unless the current conversation context is required.

Role set:
1. `user-representative` — preserve user intent, acceptance criteria, user-facing scenarios, and blockers.
2. `software-systems-engineer` — when needed, inspect OS/runtime/dependency/permission/deployment/operational constraints and system-level verification.
3. `software-designer` — produce implementation design, change scope, risks, work packages, and verification strategy.
4. `software-developer` — conditionally implement one independent work package with focused validation; multiple lanes may run in parallel when file scopes do not overlap.
5. `software-implementer` — apply or integrate approved changes and run focused validation.
6. `software-qa` — validate acceptance criteria with tests/checks/smoke evidence.
7. `software-reviewer` — review diff, quality, regressions, and completion evidence.

Base roles required for every task: `user-representative`, `software-designer`, `software-implementer`, `software-qa`, and `software-reviewer`.

Conditional roles:
- Use `software-systems-engineer` when OS/runtime/dependency/permission/deployment/operational constraints materially affect the task. This role is especially important for `cloud-hypervisor`, `virtiofsd`, manifest, start/stop, and systemd work. If skipped, document why the task does not need that analysis.
- Use `software-developer` only when the design identifies at least one independent work package; otherwise document why it was skipped and implement sequentially.

When defining validation commands for this repo, prefer the touched-surface checks that actually apply: `gofmt -w .`, `go vet ./...`, and `go test ./...` for Go changes; YAML parsing for manifests/OpenAPI files; `systemd-analyze verify contrib/cvmm@.service` for the service unit when `systemd-analyze` and host-installed `/usr/bin/cvmm` or an equivalent unit override path are available; and `go test ./...`, JSON/frontmatter/inventory checks, and `git diff --check` for `.pi`/docs-only resources.

Recommended serial planning chains:

Use one of these templates for the requirements, optional system-constraints, and design stages.

When system constraints do matter:

```json
{
  "chain": [
    {
      "agent": "user-representative",
      "task": "Summarize user intent, acceptance criteria, user-facing scenarios, constraints, and blockers for: $ARGUMENTS"
    },
    {
      "agent": "software-systems-engineer",
      "task": "Using prior outputs, inspect repository/environment constraints and provide system-level feasibility, risks, and verification for: $ARGUMENTS"
    },
    {
      "agent": "software-designer",
      "task": "Using prior outputs, including system constraints, design the implementation plan and verification strategy for: $ARGUMENTS"
    }
  ],
  "mode": "spawn"
}
```

When system constraints do not materially affect the task:

```json
{
  "chain": [
    {
      "agent": "user-representative",
      "task": "Summarize user intent, acceptance criteria, user-facing scenarios, constraints, and blockers for: $ARGUMENTS"
    },
    {
      "agent": "software-designer",
      "task": "Using the requirements output directly, design the implementation plan and verification strategy for: $ARGUMENTS. Also record why software-systems-engineer was not needed."
    }
  ],
  "mode": "spawn"
}
```

Recommended parallel development call:

Use this call only when the designer has separated non-overlapping work packages.

```json
{
  "tasks": [
    {
      "agent": "software-developer",
      "task": "Implement independent work package A for: $ARGUMENTS. Allowed files: <paths>. Acceptance criteria: <criteria>. Preserve existing behavior outside this package. Run focused validation: <commands>."
    },
    {
      "agent": "software-developer",
      "task": "Implement independent work package B for: $ARGUMENTS. Allowed files: <paths>. Acceptance criteria: <criteria>. Preserve existing behavior outside this package. Run focused validation: <commands>."
    }
  ],
  "mode": "spawn"
}
```

Recommended implementation merge call:

Use this call after any developer lanes, or use it directly when the work stays sequential.

```json
{
  "agent": "software-implementer",
  "task": "Integrate developer lane results or implement remaining approved changes for: $ARGUMENTS",
  "mode": "spawn"
}
```

Recommended parallel verification and review call:

```json
{
  "tasks": [
    {
      "agent": "software-qa",
      "task": "Validate the implementation against acceptance criteria for: $ARGUMENTS"
    },
    {
      "agent": "software-reviewer",
      "task": "Review the implementation, evidence, maintainability, and completion readiness for: $ARGUMENTS"
    }
  ],
  "mode": "spawn"
}
```

Use the parallel development call only when the designer has separated non-overlapping work packages. If there is exactly one independent package, run a single top-level `subagent({ agent, task, mode })` call for one `software-developer` lane before `software-implementer` integration. If the work cannot be safely split into a developer package, skip `software-developer` and use `software-implementer` sequentially.

If project-local subagent confirmation is required, do not try to bypass it. Ask for confirmation or fall back to role-by-role execution in the root agent.

Do not finish until every explicit requirement is mapped to current evidence from files, commands, diffs, tests, logs, or artifacts. If validation fails, triage and fix the cause before final reporting.
