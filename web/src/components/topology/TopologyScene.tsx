import { Canvas } from '@react-three/fiber';
import { OrbitControls, Line, Html } from '@react-three/drei';
import { useMemo, useState, useCallback } from 'react';
import { useAgentFenceStore, AgentNode } from '../../store/useAgentFenceStore';
import { AgentSphere } from './AgentSphere';

// Depth-based radial layout for the spawn tree
function layoutNodes(nodes: AgentNode[]): Record<string, [number, number, number]> {
  const depthOf = (id: string, map: Map<string, AgentNode>, seen = new Set<string>()): number => {
    const n = map.get(id);
    if (!n?.parentSessionId || seen.has(id)) return 0;
    seen.add(id);
    return 1 + depthOf(n.parentSessionId, map, seen);
  };

  const map = new Map(nodes.map((n) => [n.sessionId, n]));
  const byDepth: Record<number, AgentNode[]> = {};

  nodes.forEach((n) => {
    const d = depthOf(n.sessionId, map);
    (byDepth[d] ??= []).push(n);
  });

  const positions: Record<string, [number, number, number]> = {};

  Object.entries(byDepth).forEach(([depthStr, group]) => {
    const depth = Number(depthStr);
    const radius = depth * 4.5 + 2;
    group.forEach((n, i) => {
      const angle = (i / Math.max(group.length, 1)) * Math.PI * 2 - Math.PI / 2;
      positions[n.sessionId] = [
        Math.cos(angle) * radius,
        depth * -0.6,
        Math.sin(angle) * radius,
      ];
    });
  });

  return positions;
}

export function TopologyScene() {
  const nodes = useAgentFenceStore((s) => Object.values(s.nodes));
  const spawnEdges = useAgentFenceStore((s) => s.spawnEdges);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const positions = useMemo(() => layoutNodes(nodes), [nodes]);

  const handleSelect = useCallback((sessionId: string) => {
    setSelectedId((prev) => (prev === sessionId ? null : sessionId));
  }, []);

  const selectedNode = selectedId ? nodes.find((n) => n.sessionId === selectedId) : null;

  return (
    <div className="h-full w-full relative">
      <Canvas camera={{ position: [12, 10, 12], fov: 50 }}>
        <color attach="background" args={['#09090b']} />
        <ambientLight intensity={0.3} />
        <pointLight position={[10, 15, 10]} intensity={0.8} castShadow />
        <pointLight position={[-10, -5, -10]} intensity={0.3} color="#6366f1" />

        {/* Grid helper for depth perception */}
        <gridHelper args={[30, 20, '#27272a', '#18181b']} position={[0, -3, 0]} />

        {/* Root anchor */}
        <mesh position={[0, 2, 0]}>
          <sphereGeometry args={[0.3, 16, 16]} />
          <meshStandardMaterial color="#52525b" />
        </mesh>
        <Html position={[0, 2.8, 0]} center>
          <div className="text-[10px] text-zinc-500 whitespace-nowrap">root</div>
        </Html>

        {/* Spawn-tree edges */}
        {spawnEdges
          .filter((e) => positions[e.source] && positions[e.target])
          .map((edge, i) => {
            const isFenced = edge.state === 'FENCED' || edge.flagged;
            return (
              <Line
                key={`edge-${i}`}
                points={[positions[edge.source], positions[edge.target]]}
                color={isFenced ? '#ef4444' : '#52525b'}
                lineWidth={edge.flagged ? 2 : 1}
                dashed={edge.flagged}
                transparent
                opacity={edge.flagged ? 0.8 : 0.5}
              />
            );
          })}

        {/* Agent spheres */}
        {nodes.map((node) => {
          const pos = positions[node.sessionId] ?? [0, 0, 0];
          return (
            <AgentSphere
              key={node.sessionId}
              node={node}
              position={pos}
              onSelect={handleSelect}
              isSelected={selectedId === node.sessionId}
            />
          );
        })}

        <OrbitControls enableDamping dampingFactor={0.08} minDistance={4} maxDistance={45} />
      </Canvas>

      {/* Bottom-left legend */}
      <div className="absolute bottom-4 left-4 flex gap-4 text-xs text-zinc-400 bg-zinc-900/80 backdrop-blur rounded-md px-3 py-2 border border-zinc-800">
        <LegendDot color="#22c55e" label="Open" />
        <LegendDot color="#eab308" label="Half-Open" />
        <LegendDot color="#ef4444" label="Fenced" />
        <LegendDot color="#3f3f46" label="Terminated" />
      </div>

      {/* Selected node detail panel */}
      {selectedNode && (
        <div className="absolute top-4 right-4 w-72 bg-zinc-900/95 backdrop-blur rounded-lg border border-zinc-800 shadow-xl p-4 text-xs">
          <h3 className="text-sm font-semibold text-zinc-100 mb-3">
            {selectedNode.agentName || selectedNode.sessionId.slice(0, 8)}
          </h3>
          <div className="space-y-2 text-zinc-400">
            <DetailRow label="Session" value={selectedNode.sessionId.slice(0, 12) + '…'} />
            <DetailRow label="State" value={selectedNode.state} />
            <DetailRow
              label="Suspicion"
              value={`${(selectedNode.suspicionScore * 100).toFixed(1)}%`}
            />
            <DetailRow label="Intent" value={selectedNode.declaredIntent || '—'} />
            {selectedNode.parentSessionId && (
              <DetailRow label="Parent" value={selectedNode.parentSessionId.slice(0, 12) + '…'} />
            )}
          </div>
          <div className="flex gap-2 mt-4">
            <button className="flex-1 px-2 py-1.5 rounded bg-red-600 hover:bg-red-500 text-white transition-colors text-xs font-medium">
              Fence
            </button>
            <button className="flex-1 px-2 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors text-xs font-medium">
              Audit
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function LegendDot({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="w-2 h-2 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between">
      <span>{label}</span>
      <span className="text-zinc-300 font-mono">{value}</span>
    </div>
  );
}
