package agentfence

import future.keywords.if
import future.keywords.in

# === DEFAULT: DENY ===
# Every request must match an explicit allow rule.
default decision := {"outcome": "DENY", "reason": "no matching allow rule (default deny)"}

# === ALLOW: Low-risk, in-scope calls ===
# Permit calls that are not destructive, are within the agent's declared scope,
# and have a low suspicion score.
decision := {"outcome": "ALLOW", "reason": "low risk, in declared scope"} if {
    input.blast_report == null
    scope_permits(input.tool)
    input.suspicion < 0.5
}

decision := {"outcome": "ALLOW", "reason": "low risk, non-destructive with approval"} if {
    input.blast_report != null
    input.blast_report.destructive == false
    scope_permits(input.tool)
    input.suspicion < 0.5
}

# === REQUIRE APPROVAL: Destructive high-criticality actions ===
# Any destructive action with criticality >= 70 requires human approval,
# regardless of the agent's trust level.
decision := {"outcome": "REQUIRE_APPROVAL", "reason": sprintf(
    "destructive action, criticality %d — human approval required", [input.blast_report.criticality_score]
)} if {
    input.blast_report != null
    input.blast_report.destructive == true
    input.blast_report.criticality_score >= 70
}

decision := {"outcome": "REQUIRE_APPROVAL", "reason": sprintf(
    "high suspicion (%.2f) on destructive action", [input.suspicion]
)} if {
    input.blast_report != null
    input.blast_report.destructive == true
    input.suspicion >= 0.3
    input.suspicion < 0.75
}

# === DENY: Write attempted under read-only scope ===
decision := {"outcome": "DENY", "reason": "write attempted under read-only declared scope"} if {
    "read-only" in input.session.declared_scope
    is_write_tool(input.tool)
}

# === DENY: Network call under no-network scope ===
decision := {"outcome": "DENY", "reason": "network call under no-network declared scope"} if {
    "no-network" in input.session.declared_scope
    is_network_tool(input.tool)
}

# === DENY: Suspicion above fence threshold ===
decision := {"outcome": "DENY", "reason": "session suspicion above fence threshold"} if {
    input.suspicion >= 0.75
}

# === DENY: Destructive action in production without approval ===
decision := {"outcome": "DENY", "reason": "destructive action in production namespace requires approval"} if {
    input.blast_report != null
    input.blast_report.destructive == true
    input.blast_report.criticality_score >= 90
    input.blast_report.requires_approval == true
}

# === HELPERS ===

# is_write_tool identifies tools that modify state.
is_write_tool(tool) if {
    write_prefixes := {
        "kubectl.delete", "kubectl.apply", "kubectl.patch",
        "terraform.apply", "terraform.destroy",
        "shell.exec", "fs.write", "fs.delete",
        "iam.putPolicy", "iam.deleteRole", "iam.attachRolePolicy",
        "cloud.instance.terminate",
    }
    tool in write_prefixes
}

# is_network_tool identifies tools that make network calls.
is_network_tool(tool) if {
    network_tools := {"http", "curl", "wget", "net.http", "fetch", "request"}
    tool in network_tools
}

# scope_permits checks if the tool is within the agent's declared scope.
scope_permits(tool) if {
    not is_write_tool(tool)
}
scope_permits(tool) if {
    not "read-only" in input.session.declared_scope
}
scope_permits(tool) if {
    not "no-network" in input.session.declared_scope
}
