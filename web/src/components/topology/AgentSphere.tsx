import { useRef, useMemo } from 'react';
import { useFrame } from '@react-three/fiber';
import { Html } from '@react-three/drei';
import * as THREE from 'three';
import { AgentNode } from '../../store/useAgentFenceStore';

const STATE_COLOR: Record<AgentNode['state'], string> = {
  OPEN: '#22c55e',
  HALF_OPEN: '#eab308',
  FENCED: '#ef4444',
  TERMINATED: '#3f3f46',
};

const STATE_EMISSIVE: Record<AgentNode['state'], string> = {
  OPEN: '#15803d',
  HALF_OPEN: '#a16207',
  FENCED: '#991b1b',
  TERMINATED: '#27272a',
};

interface AgentSphereProps {
  node: AgentNode;
  position: [number, number, number];
  onSelect?: (sessionId: string) => void;
  isSelected?: boolean;
}

export function AgentSphere({ node, position, onSelect, isSelected }: AgentSphereProps) {
  const meshRef = useRef<THREE.Mesh>(null);
  const containmentRef = useRef<THREE.Mesh>(null);
  const pulsePhaseRef = useRef(Math.random() * Math.PI * 2);

  const color = STATE_COLOR[node.state];
  const emissive = STATE_EMISSIVE[node.state];
  const baseScale = 0.35 + node.suspicionScore * 0.4;

  useFrame(({ clock }) => {
    if (!meshRef.current) return;
    const t = clock.getElapsedTime() + pulsePhaseRef.current;
    const pulseSpeed = 1 + node.suspicionScore * 6;
    const pulseAmount = 0.08 * (0.3 + node.suspicionScore);
    const pulse = 1 + Math.sin(t * pulseSpeed) * pulseAmount;
    meshRef.current.scale.setScalar(baseScale * pulse);

    if (containmentRef.current && node.state === 'FENCED') {
      const grow = 1 + ((t * 0.6) % 1) * 1.8;
      containmentRef.current.scale.setScalar(baseScale * grow);
      const mat = containmentRef.current.material as THREE.MeshBasicMaterial;
      mat.opacity = Math.max(0, 1 - ((t * 0.6) % 1));
    }
  });

  const labelText = `${node.agentName || node.sessionId.slice(0, 8)} · ${(node.suspicionScore * 100).toFixed(0)}%`;

  return (
    <group position={position}>
      {/* Suspicion glow ring */}
      {node.suspicionScore > 0.3 && (
        <mesh>
          <ringGeometry args={[1.1, 1.3, 32]} />
          <meshBasicMaterial
            color={color}
            transparent
            opacity={Math.min(node.suspicionScore, 0.6)}
            side={THREE.DoubleSide}
          />
        </mesh>
      )}

      {/* Main sphere */}
      <mesh
        ref={meshRef}
        onClick={() => onSelect?.(node.sessionId)}
        onPointerOver={(e) => {
          (e.object as THREE.Mesh).scale.setScalar(baseScale * 1.15);
          document.body.style.cursor = 'pointer';
        }}
        onPointerOut={(e) => {
          (e.object as THREE.Mesh).scale.setScalar(baseScale);
          document.body.style.cursor = 'default';
        }}
      >
        <sphereGeometry args={[1, 32, 32]} />
        <meshStandardMaterial
          color={color}
          emissive={emissive}
          emissiveIntensity={isSelected ? 0.9 : 0.4 + node.suspicionScore * 0.4}
          metalness={0.1}
          roughness={0.6}
        />
      </mesh>

      {/* Fence containment sphere */}
      {node.state === 'FENCED' && (
        <mesh ref={containmentRef}>
          <sphereGeometry args={[1, 24, 24]} />
          <meshBasicMaterial color="#ef4444" transparent opacity={0.3} wireframe />
        </mesh>
      )}

      {/* Selection ring */}
      {isSelected && (
        <mesh>
          <ringGeometry args={[1.2, 1.5, 32]} />
          <meshBasicMaterial color="#60a5fa" transparent opacity={0.6} side={THREE.DoubleSide} />
        </mesh>
      )}

      {/* Label */}
      <Html distanceFactor={14} position={[0, baseScale + 0.7, 0]} center>
        <div className={`text-[10px] whitespace-nowrap px-2 py-0.5 rounded border ${
          node.state === 'FENCED'
            ? 'bg-red-900/80 border-red-700 text-red-200'
            : node.state === 'HALF_OPEN'
            ? 'bg-amber-900/80 border-amber-700 text-amber-200'
            : 'bg-zinc-900/80 border-zinc-700 text-zinc-200'
        }`}>
          {labelText}
        </div>
      </Html>
    </group>
  );
}
