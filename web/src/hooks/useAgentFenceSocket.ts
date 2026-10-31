import { useEffect, useRef } from 'react';
import { useAgentFenceStore } from '../store/useAgentFenceStore';

const SSE_URL = import.meta.env.VITE_AGENTFENCE_WS_URL ?? 'http://localhost:8080/ws/stream';

export function useAgentFenceSocket() {
  const { upsertNode, pushDecision, pushFenceEvent, pushSpawnEdge, setConnectionStatus } =
    useAgentFenceStore();
  const retryDelay = useRef(1000);

  useEffect(() => {
    let eventSource: EventSource;
    let closedByEffect = false;
    let reconnectTimer: ReturnType<typeof setTimeout>;

    function connect() {
      setConnectionStatus('connecting');
      const url = `${SSE_URL}?subjects=decisions,suspicion,fences,spawns,blast_radius`;
      eventSource = new EventSource(url);

      eventSource.onopen = () => {
        setConnectionStatus('connected');
        retryDelay.current = 1000;
      };

      eventSource.addEventListener('connected', (e: MessageEvent) => {
        setConnectionStatus('connected');
      });

      eventSource.addEventListener('message', (e: MessageEvent) => {
        try {
          const msg = JSON.parse(e.data);

          if ('decision_id' in msg) {
            pushDecision({
              decisionId: msg.decision_id,
              sessionId: msg.session_id ?? msg.call_id,
              toolName: msg.tool_name ?? '',
              outcome: msg.outcome,
              reason: msg.reason ?? '',
              decidedAt: msg.decided_at ?? msg.decision_time ?? new Date().toISOString(),
            });
          } else if ('fence_id' in msg) {
            pushFenceEvent({
              fenceId: msg.fence_id,
              sessionId: msg.session_id,
              level: msg.level,
              trigger: msg.trigger ?? '',
              triggeredAt: msg.triggered_at ?? msg.trigger_time ?? new Date().toISOString(),
            });
          } else if ('score' in msg) {
            upsertNode({
              sessionId: msg.session_id,
              suspicionScore: msg.score,
              agentName: msg.agent_name ?? msg.session_id?.slice(0, 8),
            });
          } else if ('child_session_id' in msg) {
            pushSpawnEdge({
              edgeId: msg.edge_id ?? crypto.randomUUID(),
              parentSessionId: msg.parent_session_id,
              childSessionId: msg.child_session_id,
              inheritedState: msg.inherited_state ?? 'OPEN',
              flagged: msg.flagged ?? false,
              createdAt: msg.created_at ?? new Date().toISOString(),
            });
            upsertNode({
              sessionId: msg.child_session_id,
              parentSessionId: msg.parent_session_id,
              state: msg.flagged ? 'HALF_OPEN' : 'OPEN',
              agentName: msg.child_session_id?.slice(0, 8),
            });
          }
        } catch {
          // ignore malformed messages
        }
      });

      eventSource.onerror = () => {
        if (closedByEffect) return;
        setConnectionStatus('disconnected');
        eventSource.close();
        reconnectTimer = setTimeout(connect, retryDelay.current);
        retryDelay.current = Math.min(retryDelay.current * 2, 15000);
      };
    }

    connect();

    return () => {
      closedByEffect = true;
      clearTimeout(reconnectTimer);
      eventSource?.close();
    };
  }, [upsertNode, pushDecision, pushFenceEvent, pushSpawnEdge, setConnectionStatus]);
}
