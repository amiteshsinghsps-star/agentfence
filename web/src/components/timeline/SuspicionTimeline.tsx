import { useMemo } from 'react';
import { AxisBottom, AxisLeft } from '@visx/axis';
import { scaleLinear, scaleTime } from '@visx/scale';
import { LinePath } from '@visx/shape';
import { useAgentFenceStore } from '../../store/useAgentFenceStore';

export function SuspicionTimeline() {
  const fenceEvents = useAgentFenceStore((s) => s.fenceEvents);
  const decisions = useAgentFenceStore((s) => s.decisions);

  const width = 960;
  const height = 360;
  const margin = { top: 20, right: 20, bottom: 35, left: 50 };

  const now = Date.now();
  const fifteenMinAgo = now - 15 * 60 * 1000;

  const xScale = useMemo(
    () =>
      scaleTime({
        domain: [new Date(fifteenMinAgo), new Date(now)],
        range: [margin.left, width - margin.right],
      }),
    [fifteenMinAgo, now],
  );

  const yScale = useMemo(
    () =>
      scaleLinear({
        domain: [0, 1],
        range: [height - margin.bottom, margin.top],
      }),
    [height],
  );

  // Simulated suspicion data points (in production, streamed from WebSocket)
  const suspicionPoints = useMemo(() => {
    const points: { time: Date; score: number }[] = [];
    for (let i = 0; i < 60; i++) {
      const t = new Date(fifteenMinAgo + (i / 60) * (now - fifteenMinAgo));
      const base = Math.sin(i / 10) * 0.2 + 0.3;
      const spike = fenceEvents.some(
        (f) => Math.abs(new Date(f.triggeredAt).getTime() - t.getTime()) < 60_000,
      )
        ? 0.4
        : 0;
      points.push({ time: t, score: Math.min(1, base + spike + Math.random() * 0.1) });
    }
    return points;
  }, [fenceEvents, fifteenMinAgo, now]);

  const OUTCOME_COLORS: Record<string, string> = {
    ALLOW: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30',
    DENY: 'text-red-400 bg-red-500/10 border-red-500/30',
    REQUIRE_APPROVAL: 'text-amber-400 bg-amber-500/10 border-amber-500/30',
    SIMULATED_BLOCK: 'text-blue-400 bg-blue-500/10 border-blue-500/30',
  };

  return (
    <div className="p-6 h-full overflow-auto">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-medium text-zinc-300">Fence Events &amp; Decisions — last 15 min</h2>
        <div className="flex items-center gap-3 text-xs text-zinc-500">
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-red-500" /> Fence
          </span>
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-emerald-500" /> Allow
          </span>
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-amber-500" /> Flag
          </span>
        </div>
      </div>

      <svg width={width} height={height} className="bg-zinc-900/50 rounded-lg border border-zinc-800">
        {/* Threshold lines */}
        <line
          x1={margin.left} x2={width - margin.right}
          y1={yScale(0.75)} y2={yScale(0.75)}
          stroke="#ef4444" strokeDasharray="4 3" opacity={0.4}
        />
        <text x={width - margin.right - 4} y={yScale(0.75) - 4} fill="#ef4444" fontSize={9} textAnchor="end">
          KILL threshold (0.75)
        </text>
        <line
          x1={margin.left} x2={width - margin.right}
          y1={yScale(0.5)} y2={yScale(0.5)}
          stroke="#eab308" strokeDasharray="4 3" opacity={0.4}
        />
        <text x={width - margin.right - 4} y={yScale(0.5) - 4} fill="#eab308" fontSize={9} textAnchor="end">
          FREEZE threshold (0.50)
        </text>

        {/* Suspicion score line */}
        <LinePath
          data={suspicionPoints}
          x={(d) => xScale(d.time) ?? 0}
          y={(d) => yScale(d.score) ?? 0}
          stroke="#22c55e"
          strokeWidth={2}
          strokeOpacity={0.8}
        />

        {/* Score fill area as a simple polygon */}
        <polygon
          points={[
            `${margin.left},${height - margin.bottom}`,
            ...suspicionPoints.map((p) => `${xScale(p.time)},${yScale(p.score)}`),
            `${width - margin.right},${height - margin.bottom}`,
          ].join(' ')}
          fill="url(#gradient)"
          opacity={0.15}
        />
        <defs>
          <linearGradient id="gradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#22c55e" />
            <stop offset="100%" stopColor="#22c55e" stopOpacity={0} />
          </linearGradient>
        </defs>

        {/* Fence event markers */}
        {fenceEvents.map((f) => {
          const x = xScale(new Date(f.triggeredAt));
          return (
            <g key={f.fenceId}>
              <line
                x1={x} x2={x}
                y1={margin.top} y2={height - margin.bottom}
                stroke={f.level === 'KILL' ? '#ef4444' : f.level === 'QUARANTINE' ? '#f97316' : '#eab308'}
                strokeWidth={1.5}
                strokeDasharray="3 2"
                opacity={0.7}
              />
              <rect x={x - 3} y={margin.top} width={6} height={6} rx={1}
                fill={f.level === 'KILL' ? '#ef4444' : f.level === 'QUARANTINE' ? '#f97316' : '#eab308'} />
              <text x={x + 5} y={margin.top + 10} fill="#ef4444" fontSize={9}>
                {f.level}
              </text>
            </g>
          );
        })}

        <AxisLeft
          scale={yScale}
          left={margin.left}
          stroke="#52525b"
          tickStroke="#52525b"
          tickFormat={((v: {valueOf(): number}) => `${(v.valueOf() * 100).toFixed(0)}%`) as any}
          tickLabelProps={() => ({ fill: '#71717a', fontSize: 9, dx: -4 })}
        />
        <AxisBottom
          scale={xScale}
          top={height - margin.bottom}
          stroke="#52525b"
          tickStroke="#52525b"
          tickFormat={((d: unknown) => {
            const date = d as Date;
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          }) as any}
          tickLabelProps={() => ({ fill: '#71717a', fontSize: 9, dy: 4 })}
          numTicks={8}
        />
      </svg>

      {/* Recent decisions list */}
      <div className="mt-4 space-y-1">
        <h3 className="text-xs font-medium text-zinc-500 mb-2 uppercase tracking-wider">Recent decisions</h3>
        {decisions.slice(0, 30).map((d) => (
          <div
            key={d.decisionId}
            className={`text-xs px-3 py-1.5 rounded flex justify-between border ${OUTCOME_COLORS[d.outcome] ?? 'text-zinc-400 bg-zinc-800/50 border-zinc-700'}`}
          >
            <span className="font-mono">{d.toolName}</span>
            <span className="flex items-center gap-2">
              <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                d.outcome === 'ALLOW' ? 'bg-emerald-500/20 text-emerald-400' :
                d.outcome === 'DENY' ? 'bg-red-500/20 text-red-400' :
                'bg-amber-500/20 text-amber-400'
              }`}>
                {d.outcome}
              </span>
              <span className="text-zinc-500 max-w-48 truncate">{d.reason}</span>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
