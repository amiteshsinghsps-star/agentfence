import { useAgentFenceSocket } from './hooks/useAgentFenceSocket';
import { ConnectionBadge } from './components/shell/ConnectionBadge';
import { useState, lazy, Suspense } from 'react';

// Code-split heavy components: Three.js (1.2MB), Monaco editor (800KB), D3/visx (200KB)
const TopologyScene = lazy(() => import('./components/topology/TopologyScene').then(m => ({ default: m.TopologyScene })));
const SuspicionTimeline = lazy(() => import('./components/timeline/SuspicionTimeline').then(m => ({ default: m.SuspicionTimeline })));
const PolicyEditor = lazy(() => import('./components/policy/PolicyEditor').then(m => ({ default: m.PolicyEditor })));
const AuditLogExplorer = lazy(() => import('./components/audit/AuditLogExplorer').then(m => ({ default: m.AuditLogExplorer })));

type Tab = 'topology' | 'timeline' | 'policy' | 'audit';

const TAB_LABELS: Record<Tab, string> = {
  topology: '3D Topology',
  timeline: 'Timeline',
  policy: 'Policy Editor',
  audit: 'Audit Trail',
};

function LoadingFallback() {
  return (
    <div className="h-full w-full flex items-center justify-center bg-zinc-950">
      <div className="flex flex-col items-center gap-3">
        <div className="w-6 h-6 border-2 border-emerald-500/30 border-t-emerald-500 rounded-full animate-spin" />
        <span className="text-xs text-zinc-500">Loading component…</span>
      </div>
    </div>
  );
}

export default function App() {
  useAgentFenceSocket();
  const [tab, setTab] = useState<Tab>('topology');

  return (
    <div className="h-screen w-screen bg-zinc-950 text-zinc-100 flex flex-col">
      {/* Header */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-zinc-800 bg-zinc-900/80 backdrop-blur">
        <div className="flex items-center gap-3">
          {/* Logo */}
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-emerald-500 to-emerald-700 flex items-center justify-center">
              <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <span className="text-lg font-semibold tracking-tight bg-gradient-to-r from-zinc-100 to-zinc-400 bg-clip-text text-transparent">
              AgentFence
            </span>
          </div>

          {/* Navigation */}
          <nav className="flex gap-1 ml-8">
            {(Object.keys(TAB_LABELS) as Tab[]).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`px-3 py-1.5 rounded-md text-sm font-medium transition-all ${
                  tab === t
                    ? 'bg-emerald-500/15 text-emerald-400 shadow-sm'
                    : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50'
                }`}
              >
                {TAB_LABELS[t]}
              </button>
            ))}
          </nav>
        </div>

        <div className="flex items-center gap-4">
          {/* Pending approvals badge */}
          <button className="relative px-2 py-1 rounded text-xs text-amber-400 bg-amber-500/10 border border-amber-500/30 hover:bg-amber-500/20 transition-colors">
            <span className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              2 pending
            </span>
          </button>
          <ConnectionBadge />
        </div>
      </header>

      {/* Main content with lazy loading */}
      <main className="flex-1 min-h-0">
        <Suspense fallback={<LoadingFallback />}>
          {tab === 'topology' && <TopologyScene />}
          {tab === 'timeline' && <SuspicionTimeline />}
          {tab === 'policy' && <PolicyEditor />}
          {tab === 'audit' && <AuditLogExplorer />}
        </Suspense>
      </main>
    </div>
  );
}
