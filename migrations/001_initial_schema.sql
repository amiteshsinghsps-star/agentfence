-- AgentFence PostgreSQL Schema v0.1.0
-- Hash-chained audit trail, session management, and spawn graph.

-- Agents & Sessions -----------------------------------------------------

CREATE TABLE IF NOT EXISTS agents (
    agent_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name    TEXT NOT NULL,
    framework       TEXT NOT NULL,             -- e.g. 'claude-code', 'langgraph', 'custom-mcp'
    spiffe_id       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    session_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id            UUID NOT NULL REFERENCES agents(agent_id),
    parent_session_id   UUID REFERENCES sessions(session_id),
    declared_intent     TEXT NOT NULL,
    declared_scope_tags TEXT[] NOT NULL DEFAULT '{}',
    state               TEXT NOT NULL DEFAULT 'OPEN'
                          CHECK (state IN ('OPEN','HALF_OPEN','FENCED','TERMINATED')),
    suspicion_score     NUMERIC(6,3) NOT NULL DEFAULT 0,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_state  ON sessions(state);
CREATE INDEX IF NOT EXISTS idx_sessions_agent  ON sessions(agent_id);

-- Tool calls & decisions --------------------------------------------------

CREATE TABLE IF NOT EXISTS tool_calls (
    call_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    tool_name       TEXT NOT NULL,
    tool_server     TEXT NOT NULL DEFAULT '',
    arguments       JSONB NOT NULL DEFAULT '{}',
    blast_class     TEXT NOT NULL DEFAULT 'low'
                     CHECK (blast_class IN ('low','medium','high','critical')),
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tool_calls_session ON tool_calls(session_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_time    ON tool_calls(requested_at DESC);

CREATE TABLE IF NOT EXISTS decisions (
    decision_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    call_id           UUID NOT NULL REFERENCES tool_calls(call_id) ON DELETE CASCADE,
    session_id        UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    policy_version    TEXT NOT NULL DEFAULT 'unknown',
    outcome           TEXT NOT NULL
                       CHECK (outcome IN ('ALLOW','DENY','REQUIRE_APPROVAL','SIMULATED_BLOCK')),
    reason            TEXT NOT NULL DEFAULT '',
    blast_report      JSONB,             -- BlastRadiusReport, nullable
    suspicion_at_time NUMERIC(6,3) NOT NULL DEFAULT 0,
    decided_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    prev_hash         TEXT NOT NULL,
    this_hash         TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_decisions_call     ON decisions(call_id);
CREATE INDEX IF NOT EXISTS idx_decisions_session  ON decisions(session_id);
CREATE INDEX IF NOT EXISTS idx_decisions_outcome  ON decisions(outcome);
CREATE INDEX IF NOT EXISTS idx_decisions_time     ON decisions(decided_at DESC);

-- Fencing -----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS fence_events (
    fence_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    level         TEXT NOT NULL CHECK (level IN ('FREEZE','QUARANTINE','KILL')),
    trigger       TEXT NOT NULL,          -- 'suspicion_threshold' | 'policy_hard_deny' | 'manual' | 'propagation'
    triggered_by  TEXT,                   -- user id if manual
    trigger_reason TEXT NOT NULL DEFAULT '',
    triggered_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at    TIMESTAMPTZ,
    cleared_by    TEXT
);

CREATE INDEX IF NOT EXISTS idx_fence_events_session ON fence_events(session_id);
CREATE INDEX IF NOT EXISTS idx_fence_events_time    ON fence_events(triggered_at DESC);

-- Spawn / propagation graph -------------------------------------------------

CREATE TABLE IF NOT EXISTS spawn_edges (
    edge_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_session_id UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    child_session_id  UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    inherited_state   TEXT NOT NULL DEFAULT 'OPEN',      -- state parent was in at spawn time
    flagged           BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(child_session_id)
);

CREATE INDEX IF NOT EXISTS idx_spawn_edges_parent ON spawn_edges(parent_session_id);
CREATE INDEX IF NOT EXISTS idx_spawn_edges_child  ON spawn_edges(child_session_id);

-- Chain verification status -------------------------------------------------

CREATE TABLE IF NOT EXISTS chain_verifications (
    verification_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    verified_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    start_decision_id UUID NOT NULL REFERENCES decisions(decision_id),
    end_decision_id   UUID NOT NULL REFERENCES decisions(decision_id),
    is_valid          BOOLEAN NOT NULL,
    error_message     TEXT
);

CREATE INDEX IF NOT EXISTS idx_chain_verifications_time ON chain_verifications(verified_at DESC);

-- Export tracking -----------------------------------------------------------

CREATE TABLE IF NOT EXISTS audit_exports (
    export_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    record_count  INTEGER NOT NULL,
    checksum      TEXT NOT NULL,
    storage_path  TEXT NOT NULL
);

-- Row-level security (optional, for multi-tenant deployments)
-- ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE decisions ENABLE ROW LEVEL SECURITY;
