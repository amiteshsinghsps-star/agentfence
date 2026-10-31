# AgentFence

Runtime fencing and policy control plane for autonomous AI agents acting on real infrastructure. It sits at the MCP boundary and decides, per tool call, whether the action executes.

## Problem

- Agents execute shell, cloud CLI, Kubernetes and Terraform actions directly, at machine speed.
- RBAC authorizes identities, not intent. A crafted prompt can redirect an agent without stealing any credential.
- Static policy-as-code and API gateways do not model behavioral drift within a session.
- Unsafe state can propagate from parent to child agents through spawn trees.

## How It Works

Every MCP tool call passes through a transparent proxy (no SDK changes):

1. Reject if the session is already fenced.
2. Score behavior against the session's declared intent.
3. Classify blast radius; run a dry-run simulation for high-risk calls.
4. Evaluate Rego policy: `ALLOW`, `DENY` or `REQUIRE_APPROVAL`.
5. Append the decision to a hash-chained audit log.
6. Fence the session if suspicion crosses a threshold.

Compatible with any MCP client (Claude Code, Cursor, LangGraph, AutoGen).

## Design

| Mechanism | Detail | Basis |
|---|---|---|
| Intent-bound sessions | Declared intent and scope tags (`read-only`, `no-network`) set at creation | Makes misbehavior a checkable contract |
| Continuous suspicion score | Phi-accrual-style, exponential decay, 5 min half-life | Distributed failure detection; catches gradual drift |
| Escalating fencing | Freeze (≥0.50, auto-recovers) → Quarantine (≥0.75) → Kill | STONITH cluster fencing |
| Blast-radius simulation | K8s server-side dry-run; Terraform plan parsing | Judges effect, not tool name |
| Propagation breaker | `FENCED` parent → `HALF_OPEN` child; scope escalation flagged | Contains spawn-tree contagion |
| Tamper-evident audit | `this_hash = SHA256(prev_hash ‖ canonical_body)` | Detects retroactive edits |
| Fail-closed | Any internal failure → `DENY` | Safety over availability |

Suspicion signals: scope violation +0.50, novel tool +0.15 (early) / +0.05 (late), argument entropy +0.2×entropy, calls <50 ms apart +0.05.

Session states: `OPEN → HALF_OPEN → FENCED → TERMINATED`.

## Components

| Path | Role |
|---|---|
| `cmd/proxy` | MCP interception and decision pipeline |
| `cmd/wsgateway` | SSE event stream for the dashboard |
| `cmd/apiserver` | REST control plane |
| `internal/session` | Thread-safe session state |
| `internal/policy` | Policy evaluation, static blast classification |
| `internal/fencedetect` | Suspicion scoring |
| `internal/fence` | Fence enforcement |
| `internal/simulate` | Kubernetes and Terraform simulators |
| `internal/propagation` | Spawn-tree containment |
| `internal/audit` | Hash-chained decision log |
| `internal/bus` | Event bus (in-memory; NATS JetStream via build tag) |
| `web/` | React 19 dashboard: 3D topology, suspicion timeline, Rego editor, audit explorer |

## Quick Start

Requires Go 1.23+, Node 22+. Docker optional.

```bash
go build -o bin/proxy ./cmd/proxy/
go build -o bin/wsgateway ./cmd/wsgateway/
go build -o bin/apiserver ./cmd/apiserver/

./bin/proxy &        # :8443
./bin/wsgateway &    # :8080
./bin/apiserver &    # :8081

docker-compose up    # full stack: services, dashboard, postgres, redis, nats
```

Tests and checks:

```bash
go vet ./... && go test -count=1 ./...   # 16 tests (fencedetect 11, audit 5)
helm lint charts/agentfence/
```

## API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/sessions`, `/api/v1/sessions/{id}` | List / inspect sessions |
| `POST` | `/api/v1/sessions/{id}/fence`, `/unfence` | Manual fence control (unfence requires a reason) |
| `GET` `PUT` | `/api/v1/policy` | Read / publish policy |
| `POST` | `/api/v1/policy/dry-run` | Dry-run a candidate policy |
| `POST` | `/api/v1/simulate` | Blast-radius simulation |
| `GET` | `/api/v1/decisions`, `/topology`, `/audit/export` | Decisions, spawn graph, audit export |
| `SSE` | `/ws/stream?subjects=decisions,suspicion,fences,spawns` | Live events |

## Policy Example

```rego
package agentfence

default decision := {"outcome": "DENY", "reason": "no matching allow rule"}

decision := {"outcome": "ALLOW", "reason": "low risk, in scope"} if {
    input.blast_report == null
    scope_permits(input.tool)
    input.suspicion < 0.5
}

decision := {"outcome": "REQUIRE_APPROVAL", "reason": "destructive, high criticality"} if {
    input.blast_report.destructive == true
    input.blast_report.criticality_score >= 70
}

decision := {"outcome": "DENY", "reason": "write under read-only"} if {
    "read-only" in input.session.declared_scope
    is_write_tool(input.tool)
}
```

## Security

- Fail-closed on internal errors.
- Least-privilege ServiceAccounts and default-deny NetworkPolicies in Helm.
- Manual overrides are rate-limited and require a reason.
- Policy bundles must pass dry-run before publish.

## Roadmap

| Version | Scope | Target |
|---|---|---|
| v0.1 | MCP proxy, policy engine, audit trail | Current |
| v0.2 | Blast-radius dry-run (K8s, Terraform) | Q3 2026 |
| v0.3 | Behavioral fencing | Q3 2026 |
| v0.4 | Propagation breaker, audit export | Q4 2026 |
| v1.0 | Adversarial benchmark suite, Helm GA | Q1 2027 |

## License

Apache 2.0. See [LICENSE](LICENSE).
