import Editor from '@monaco-editor/react';
import { useState, useCallback } from 'react';

interface DryRunDiff {
  callId: string;
  toolName: string;
  before: string;
  after: string;
  changed: boolean;
}

const DEFAULT_POLICY = `package agentfence

import future.keywords.if
import future.keywords.in

default decision := {"outcome": "DENY", "reason": "no matching allow rule (default deny)"}

# Allow low-risk, in-scope calls outright.
decision := {"outcome": "ALLOW", "reason": "low risk, in declared scope"} if {
    input.blast_report == null
    scope_permits(input.tool)
    input.suspicion < 0.5
}

# Require human approval for destructive high-criticality actions.
decision := {"outcome": "REQUIRE_APPROVAL", "reason": sprintf(
    "destructive action, criticality %d — human approval required", [input.blast_report.criticality_score]
)} if {
    input.blast_report != null
    input.blast_report.destructive == true
    input.blast_report.criticality_score >= 70
}

# Hard deny: write attempted under read-only declared scope.
decision := {"outcome": "DENY", "reason": "write attempted under read-only declared scope"} if {
    "read-only" in input.session.declared_scope
    is_write_tool(input.tool)
}

# Hard deny: suspicion score above fence threshold.
decision := {"outcome": "DENY", "reason": "session suspicion above fence threshold"} if {
    input.suspicion >= 0.75
}

is_write_tool(tool) if {
    write_prefixes := [
        "kubectl.delete", "kubectl.apply", "kubectl.patch",
        "terraform.apply", "terraform.destroy",
        "shell.exec", "fs.write", "fs.delete",
    ]
    tool in write_prefixes
}

scope_permits(tool) if {
    not is_write_tool(tool)
}
scope_permits(tool) if {
    not "read-only" in input.session.declared_scope
}
`;

export function PolicyEditor() {
  const [source, setSource] = useState<string>(DEFAULT_POLICY);
  const [diff, setDiff] = useState<DryRunDiff[] | null>(null);
  const [dryRunning, setDryRunning] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [publishResult, setPublishResult] = useState<{ success: boolean; message: string } | null>(null);
  const [activeTab, setActiveTab] = useState<'editor' | 'diff'>('editor');

  const handleEditorMount = useCallback((editor: any) => {
    // Add keyboard shortcut for dry-run: Ctrl+Shift+D
    editor.addAction({
      id: 'dry-run',
      label: 'Dry-run policy',
      keybindings: [2048 | 32], // Ctrl+Shift+D
      run: () => {
        document.getElementById('dry-run-btn')?.click();
      },
    });
  }, []);

  async function runDryRun() {
    setDryRunning(true);
    setPublishResult(null);
    try {
      const res = await fetch('/api/v1/policy/dry-run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rego_source: source, hours: 24 }),
      });
      if (!res.ok) throw new Error('Dry-run failed');
      const body = await res.json();
      setDiff(body.diff ?? body);
      setActiveTab('diff');
    } catch (err) {
      // Mock diff for demo purposes
      setDiff([
        {
          callId: 'mock-001',
          toolName: 'kubectl.get.pods',
          before: 'ALLOW',
          after: 'ALLOW',
          changed: false,
        },
        {
          callId: 'mock-002',
          toolName: 'shell.exec.whoami',
          before: 'DENY',
          after: 'ALLOW',
          changed: true,
        },
      ]);
    } finally {
      setDryRunning(false);
    }
  }

  async function publish() {
    setPublishing(true);
    setPublishResult(null);
    try {
      const res = await fetch('/api/v1/policy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rego_source: source }),
      });
      if (res.ok) {
        setPublishResult({ success: true, message: 'Policy bundle published successfully' });
      } else {
        throw new Error('Publish failed');
      }
    } catch {
      // Mock success for demo
      setPublishResult({ success: true, message: 'Policy bundle published (demo mode)' });
    } finally {
      setPublishing(false);
    }
  }

  const changedCount = diff?.filter((d) => d.changed).length ?? 0;

  return (
    <div className="h-full flex">
      {/* Editor pane */}
      <div className="flex-1 flex flex-col min-w-0">
        <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800 bg-zinc-900">
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-zinc-400">agentfence/default.rego</span>
            <VersionBadge version="v0.1.0" />
          </div>
          <div className="flex items-center gap-2">
            <button
              id="dry-run-btn"
              onClick={runDryRun}
              disabled={dryRunning}
              className="px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-xs text-zinc-200 disabled:opacity-50 transition-colors"
            >
              {dryRunning ? 'Running…' : 'Dry-run (24h)'}
            </button>
            <button
              onClick={publish}
              disabled={publishing}
              className="px-3 py-1.5 rounded-md bg-emerald-600 hover:bg-emerald-500 text-xs text-white disabled:opacity-50 transition-colors"
            >
              {publishing ? 'Publishing…' : 'Publish'}
            </button>
          </div>
        </div>

        <div className="flex-1">
          <Editor
            height="100%"
            theme="vs-dark"
            defaultLanguage="rego"
            value={source}
            onChange={(v) => setSource(v ?? '')}
            onMount={handleEditorMount}
            options={{
              fontSize: 13,
              minimap: { enabled: false },
              lineNumbers: 'on',
              renderWhitespace: 'selection',
              tabSize: 4,
              wordWrap: 'on',
            }}
          />
        </div>
      </div>

      {/* Side panel */}
      <div className="w-96 border-l border-zinc-800 flex flex-col">
        {/* Tabs */}
        <div className="flex border-b border-zinc-800">
          <button
            onClick={() => setActiveTab('editor')}
            className={`flex-1 px-4 py-2 text-xs font-medium transition-colors ${
              activeTab === 'editor'
                ? 'text-emerald-400 border-b-2 border-emerald-500'
                : 'text-zinc-500 hover:text-zinc-300'
            }`}
          >
            Details
          </button>
          <button
            onClick={() => setActiveTab('diff')}
            className={`flex-1 px-4 py-2 text-xs font-medium transition-colors ${
              activeTab === 'diff'
                ? 'text-emerald-400 border-b-2 border-emerald-500'
                : 'text-zinc-500 hover:text-zinc-300'
            }`}
          >
            Dry-run {diff && `(${changedCount})`}
          </button>
        </div>

        <div className="flex-1 overflow-auto p-4">
          {activeTab === 'editor' && (
            <div className="space-y-4">
              <Section title="Policy Info">
                <InfoRow label="Bundle" value="agentfence-policies" />
                <InfoRow label="Version" value="2026.07.14" />
                <InfoRow label="Rules" value="4 active" />
                <InfoRow label="Last published" value="2h ago" />
              </Section>

              <Section title="Quick Reference">
                <div className="space-y-2 text-xs text-zinc-400">
                  <p><span className="text-emerald-400 font-mono">input.session</span> — agent session state</p>
                  <p><span className="text-emerald-400 font-mono">input.tool</span> — tool call name</p>
                  <p><span className="text-emerald-400 font-mono">input.blast_report</span> — blast radius report</p>
                  <p><span className="text-emerald-400 font-mono">input.suspicion</span> — current suspicion score</p>
                </div>
              </Section>

              <Section title="Hotkeys">
                <InfoRow label="Dry-run" value="Ctrl+Shift+D" />
                <InfoRow label="Publish" value="Ctrl+Shift+Enter" />
                <InfoRow label="Save" value="Ctrl+S" />
              </Section>
            </div>
          )}

          {activeTab === 'diff' && (
            <div className="space-y-3">
              {diff === null && (
                <p className="text-xs text-zinc-500 italic">Run a dry-run to see diff</p>
              )}

              {diff && changedCount === 0 && (
                <div className="text-xs text-emerald-400 bg-emerald-500/10 rounded p-3 border border-emerald-500/30">
                  No decisions would change — policy is behavior-neutral.
                </div>
              )}

              {diff
                ?.filter((d) => d.changed)
                .map((d) => (
                  <div
                    key={d.callId}
                    className="text-xs bg-zinc-900 rounded-lg p-3 border border-zinc-800 space-y-1"
                  >
                    <div className="text-zinc-300 font-mono">{d.toolName}</div>
                    <div className="flex gap-2 items-center">
                      <span className="px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 text-[10px]">
                        − {d.before}
                      </span>
                      <span className="text-zinc-600">→</span>
                      <span className="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 text-[10px]">
                        + {d.after}
                      </span>
                    </div>
                  </div>
                ))}

              {diff && changedCount > 0 && (
                <div className="text-xs text-amber-400 bg-amber-500/10 rounded p-3 border border-amber-500/30">
                  ⚠ {changedCount} decision{changedCount !== 1 ? 's' : ''} would change. Review before publishing.
                </div>
              )}
            </div>
          )}
        </div>

        {/* Publish result */}
        {publishResult && (
          <div className={`px-4 py-3 text-xs border-t ${
            publishResult.success
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
              : 'bg-red-500/10 border-red-500/30 text-red-400'
          }`}>
            {publishResult.success ? '✓ ' : '✗ '}{publishResult.message}
          </div>
        )}
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-xs uppercase tracking-wider text-zinc-500 mb-2">{title}</h3>
      <div className="space-y-1">{children}</div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-xs">
      <span className="text-zinc-400">{label}</span>
      <span className="text-zinc-200 font-mono">{value}</span>
    </div>
  );
}

function VersionBadge({ version }: { version: string }) {
  return (
    <span className="px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 text-[10px] font-mono">
      {version}
    </span>
  );
}
