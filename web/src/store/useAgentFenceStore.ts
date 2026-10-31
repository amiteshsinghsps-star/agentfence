import { create } from 'zustand';

export type SessionState = 'OPEN' | 'HALF_OPEN' | 'FENCED' | 'TERMINATED';

export interface AgentNode {
  sessionId: string;
  agentName: string;
  parentSessionId?: string;
  state: SessionState;
  suspicionScore: number;
  declaredIntent: string;
  lastCallAt?: string;
}

export interface DecisionEvent {
  decisionId: string;
  sessionId: string;
  toolName: string;
  outcome: 'ALLOW' | 'DENY' | 'REQUIRE_APPROVAL' | 'SIMULATED_BLOCK';
  reason: string;
  decidedAt: string;
}

export interface FenceEvent {
  fenceId: string;
  sessionId: string;
  level: 'FREEZE' | 'QUARANTINE' | 'KILL';
  trigger: string;
  triggeredAt: string;
}

export interface BlastRadiusEvent {
  callId: string;
  sessionId: string;
  toolName: string;
  arguments: Record<string, unknown>;
  affectedResources: number;
  criticalityScore: number;
  destructive: boolean;
  requestedAt: string;
}

export interface SpawnEvent {
  edgeId: string;
  parentSessionId: string;
  childSessionId: string;
  inheritedState: string;
  flagged: boolean;
  createdAt: string;
}

export interface TopologyEdge {
  source: string;
  target: string;
  state: string;
  flagged: boolean;
}

interface AgentFenceStore {
  nodes: Record<string, AgentNode>;
  decisions: DecisionEvent[];
  fenceEvents: FenceEvent[];
  blastRadiusQueue: BlastRadiusEvent[];
  spawnEdges: TopologyEdge[];
  connectionStatus: 'connecting' | 'connected' | 'disconnected';

  upsertNode: (node: Partial<AgentNode> & { sessionId: string }) => void;
  removeNode: (sessionId: string) => void;
  pushDecision: (d: DecisionEvent) => void;
  pushFenceEvent: (f: FenceEvent) => void;
  pushBlastRadius: (b: BlastRadiusEvent) => void;
  removeBlastRadius: (callId: string) => void;
  pushSpawnEdge: (e: SpawnEvent) => void;
  setConnectionStatus: (s: AgentFenceStore['connectionStatus']) => void;
}

export const useAgentFenceStore = create<AgentFenceStore>((set) => ({
  nodes: {},
  decisions: [],
  fenceEvents: [],
  blastRadiusQueue: [],
  spawnEdges: [],
  connectionStatus: 'connecting',

  upsertNode: (partial) =>
    set((s) => ({
      nodes: {
        ...s.nodes,
        [partial.sessionId]: { ...s.nodes[partial.sessionId], ...partial } as AgentNode,
      },
    })),

  removeNode: (sessionId) =>
    set((s) => {
      const { [sessionId]: _, ...rest } = s.nodes;
      return { nodes: rest };
    }),

  pushDecision: (d) =>
    set((s) => ({
      decisions: [d, ...s.decisions].slice(0, 500),
    })),

  pushFenceEvent: (f) =>
    set((s) => ({
      fenceEvents: [f, ...s.fenceEvents].slice(0, 200),
      nodes: {
        ...s.nodes,
        [f.sessionId]: { ...s.nodes[f.sessionId], state: 'FENCED' } as AgentNode,
      },
    })),

  pushBlastRadius: (b) =>
    set((s) => ({
      blastRadiusQueue: [b, ...s.blastRadiusQueue].slice(0, 50),
    })),

  removeBlastRadius: (callId) =>
    set((s) => ({
      blastRadiusQueue: s.blastRadiusQueue.filter((b) => b.callId !== callId),
    })),

  pushSpawnEdge: (e) =>
    set((s) => ({
      spawnEdges: [
        { source: e.parentSessionId, target: e.childSessionId, state: e.inheritedState, flagged: e.flagged },
        ...s.spawnEdges,
      ].slice(0, 200),
    })),

  setConnectionStatus: (connectionStatus) => set({ connectionStatus }),
}));
