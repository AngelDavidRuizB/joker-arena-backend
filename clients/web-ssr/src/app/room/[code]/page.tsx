'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from '../../../store/session';
import { Table } from '../../../views/Table';

/** La mesa es interactiva (WebSocket): se renderiza en el cliente, solo con sesión. */
export default function RoomPage() {
  const player = useSession((s) => s.player);
  const router = useRouter();
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => setHydrated(useSession.persist.hasHydrated() || true), []);
  useEffect(() => {
    if (hydrated && !player) router.replace('/');
  }, [hydrated, player, router]);
  if (!hydrated || !player) return null;
  return <Table />;
}
