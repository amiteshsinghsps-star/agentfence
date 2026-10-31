import { useAgentFenceStore } from '../../store/useAgentFenceStore';

const STATUS_CONFIG = {
  connected: { color: 'bg-emerald-500', label: 'Connected' },
  connecting: { color: 'bg-amber-500', label: 'Connecting…' },
  disconnected: { color: 'bg-red-500', label: 'Disconnected' },
} as const;

export function ConnectionBadge() {
  const status = useAgentFenceStore((s) => s.connectionStatus);
  const config = STATUS_CONFIG[status];

  return (
    <div className="flex items-center gap-2 text-xs text-zinc-400">
      <span className={`w-2 h-2 rounded-full ${config.color} animate-pulse`} />
      <span>{config.label}</span>
    </div>
  );
}
