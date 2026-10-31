import { useState, useMemo } from 'react';
import { useAgentFenceStore } from '../../store/useAgentFenceStore';

interface AuditRecord {
  id: string;
  sessionId: string;
  toolName: string;
  outcome: string;
  reason: string;
  suspicionAtTime: number;
  decidedAt: string;
  prevHash: string;
  thisHash: string;
  verified: boolean | null;
}

const MOCK_AUDIT_RECORDS: AuditRecord[] = Array.from({ length: 50 }, (_, i) => ({
  id: `dec-${String(i).padStart(6, '0')}`,
  sessionId: `sess-${Math.random().toString(16).slice(2, 10)}`,
  toolName: ['kubectl.get.pods', 'terraform.plan', 'fs.read', 'shell.exec', 'kubectl.delete', 'iam.getPolicy'][Math.floor(Math.random() * 6)],
  outcome: ['ALLOW', 'DENY', 'REQUIRE_APPROVAL'][Math.floor(Math.random() * 3)],
  reason: ['policy match', 'scope violation', 'suspicion threshold'][Math.floor(Math.random() * 3)],
  suspicionAtTime: Math.round(Math.random() * 100) / 100,
  decidedAt: new Date(Date.now() - Math.random() * 24 * 60 * 60 * 1000).toISOString(),
  prevHash: i === 0 ? 'GENESIS' : `0x${Math.random().toString(16).slice(2, 10)}…`,
  thisHash: `0x${Math.random().toString(16).slice(2, 10)}…`,
  verified: null,
}));

const OUTCOME_BADGE: Record<string, string> = {
  ALLOW: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30',
  DENY: 'bg-red-500/20 text-red-400 border-red-500/30',
  REQUIRE_APPROVAL: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
};

export function AuditLogExplorer() {
  const decisions = useAgentFenceStore((s) => s.decisions);
  const [search, setSearch] = useState('');
  const [outcomeFilter, setOutcomeFilter] = useState<string | null>(null);
  const [verifyingId, setVerifyingId] = useState<string | null>(null);
  const [records, setRecords] = useState<AuditRecord[]>(MOCK_AUDIT_RECORDS);
  const [exporting, setExporting] = useState(false);

  // Merge live decisions into mock records for demo
  const allRecords = useMemo(() => {
    const liveRecords: AuditRecord[] = decisions.map((d, i) => ({
      id: d.decisionId,
      sessionId: d.sessionId,
      toolName: d.toolName,
      outcome: d.outcome,
      reason: d.reason,
      suspicionAtTime: 0,
      decidedAt: d.decidedAt,
      prevHash: i === 0 ? 'GENESIS' : `0x${i}…`,
      thisHash: `0x${i + 1}…`,
      verified: null,
    }));
    return [...liveRecords, ...records];
  }, [decisions, records]);

  const filtered = useMemo(() => {
    return allRecords.filter((r) => {
      const matchesSearch =
        !search ||
        r.id.toLowerCase().includes(search.toLowerCase()) ||
        r.toolName.toLowerCase().includes(search.toLowerCase()) ||
        r.sessionId.toLowerCase().includes(search.toLowerCase());
      const matchesOutcome = !outcomeFilter || r.outcome === outcomeFilter;
      return matchesSearch && matchesOutcome;
    });
  }, [allRecords, search, outcomeFilter]);

  const verifyChain = async (recordId: string) => {
    setVerifyingId(recordId);
    // Simulate verification delay
    await new Promise((resolve) => setTimeout(resolve, 500));
    setRecords((prev) =>
      prev.map((r) =>
        r.id === recordId
          ? { ...r, verified: Math.random() > 0.15 } // 85% pass rate for demo
          : r,
      ),
    );
    setVerifyingId(null);
  };

  const exportAudit = async () => {
    setExporting(true);
    await new Promise((resolve) => setTimeout(resolve, 1000));
    const blob = new Blob(
      [JSON.stringify(allRecords.slice(0, 100), null, 2)],
      { type: 'application/json' },
    );
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `agentfence-audit-${new Date().toISOString().slice(0, 10)}.ndjson`;
    a.click();
    URL.revokeObjectURL(url);
    setExporting(false);
  };

  return (
    <div className="p-6 h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-medium text-zinc-300">Audit Trail</h2>
        <button
          onClick={exportAudit}
          disabled={exporting}
          className="px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-xs text-zinc-200 disabled:opacity-50 transition-colors"
        >
          {exporting ? 'Exporting…' : 'Export NDJSON'}
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1 max-w-md">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            placeholder="Search by ID, tool, or session…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-9 pr-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-600 transition-colors"
          />
        </div>
        <div className="flex gap-1">
          {(['ALLOW', 'DENY', 'REQUIRE_APPROVAL'] as const).map((o) => (
            <button
              key={o}
              onClick={() => setOutcomeFilter(outcomeFilter === o ? null : o)}
              className={`px-2 py-1 rounded text-[10px] font-medium border transition-colors ${
                outcomeFilter === o
                  ? OUTCOME_BADGE[o] + ' border-current'
                  : 'text-zinc-500 border-zinc-800 hover:border-zinc-600'
              }`}
            >
              {o === 'REQUIRE_APPROVAL' ? 'APPROVAL' : o}
            </button>
          ))}
        </div>
        <span className="text-xs text-zinc-500">{filtered.length} records</span>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto rounded-lg border border-zinc-800">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-zinc-900 text-zinc-400 uppercase tracking-wider text-[10px]">
              <th className="text-left px-3 py-2 font-medium">Time</th>
              <th className="text-left px-3 py-2 font-medium">Tool Call</th>
              <th className="text-left px-3 py-2 font-medium">Outcome</th>
              <th className="text-left px-3 py-2 font-medium">Reason</th>
              <th className="text-left px-3 py-2 font-medium">Chain</th>
              <th className="text-left px-3 py-2 font-medium">Verify</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((r) => (
              <tr key={r.id} className="border-t border-zinc-800 hover:bg-zinc-900/50 transition-colors">
                <td className="px-3 py-2 text-zinc-400 font-mono">
                  {new Date(r.decidedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                </td>
                <td className="px-3 py-2">
                  <div className="flex flex-col">
                    <span className="text-zinc-200 font-mono">{r.toolName}</span>
                    <span className="text-zinc-500 text-[10px]">{r.sessionId.slice(0, 12)}…</span>
                  </div>
                </td>
                <td className="px-3 py-2">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium border ${OUTCOME_BADGE[r.outcome] ?? ''}`}>
                    {r.outcome}
                  </span>
                </td>
                <td className="px-3 py-2 text-zinc-400 max-w-40 truncate">{r.reason}</td>
                <td className="px-3 py-2">
                  <span className="font-mono text-[10px] text-zinc-500" title={`prev: ${r.prevHash}\nthis: ${r.thisHash}`}>
                    {r.thisHash.slice(0, 12)}…
                  </span>
                </td>
                <td className="px-3 py-2">
                  {r.verified === null ? (
                    <button
                      onClick={() => verifyChain(r.id)}
                      disabled={verifyingId === r.id}
                      className="text-zinc-500 hover:text-zinc-300 disabled:opacity-50 transition-colors"
                    >
                      {verifyingId === r.id ? (
                        <span className="animate-pulse">…</span>
                      ) : (
                        'Verify'
                      )}
                    </button>
                  ) : r.verified ? (
                    <span className="text-emerald-400 flex items-center gap-1">
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                      </svg>
                      Verified
                    </span>
                  ) : (
                    <span className="text-red-400 flex items-center gap-1">
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                      Broken
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Chain integrity summary */}
      <div className="mt-3 flex items-center gap-4 text-xs text-zinc-500 bg-zinc-900/50 rounded-lg px-4 py-2 border border-zinc-800">
        <span className="flex items-center gap-1">
          <span className="w-2 h-2 rounded-full bg-emerald-500" />
          {records.filter((r) => r.verified === true).length} verified
        </span>
        <span className="flex items-center gap-1">
          <span className="w-2 h-2 rounded-full bg-red-500" />
          {records.filter((r) => r.verified === false).length} broken
        </span>
        <span className="flex items-center gap-1">
          <span className="w-2 h-2 rounded-full bg-zinc-600" />
          {records.filter((r) => r.verified === null).length} unverified
        </span>
        <span className="ml-auto text-[10px] text-zinc-600 font-mono">
          SHA256 hash-chain • append-only
        </span>
      </div>
    </div>
  );
}
