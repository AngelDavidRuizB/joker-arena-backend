import { useEffect, useRef, useState, useCallback } from "react";
import { WS_METRICS_URL } from "../config";
import type { Snapshot } from "../types/snapshot";

type WSStatus = "connecting" | "connected" | "reconnecting" | "disconnected";

interface UseClusterWSReturn {
  snapshot: Snapshot | null;
  status: WSStatus;
  lastUpdated: Date | null;
  disconnect: () => void;
  reconnect: () => void;
}

const BASE_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

/**
 * Opens a WebSocket to ws://localhost:8080/admin/cluster/metrics?token=<JWT>
 * and reconnects with exponential back-off on failure.
 *
 * Pass token=null to stay disconnected (e.g. user not yet authenticated).
 */
export function useClusterWS(token: string | null): UseClusterWSReturn {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [status, setStatus] = useState<WSStatus>("disconnected");
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const backoffRef = useRef<number>(BASE_BACKOFF_MS);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const manualDisconnect = useRef(false);

  const cleanup = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    if (wsRef.current) {
      wsRef.current.onclose = null; // prevent reconnect loop
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    if (!token) return;
    cleanup();
    manualDisconnect.current = false;
    setStatus("connecting");

    const url = `${WS_METRICS_URL}?token=${encodeURIComponent(token)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      backoffRef.current = BASE_BACKOFF_MS; // reset on success
      setStatus("connected");
    };

    ws.onmessage = (event) => {
      try {
        const data: Snapshot = JSON.parse(event.data as string);
        setSnapshot(data);
        setLastUpdated(new Date());
      } catch {
        console.warn("[useClusterWS] Failed to parse message:", event.data);
      }
    };

    ws.onerror = () => {
      // onerror always precedes onclose; let onclose handle reconnect
      console.warn("[useClusterWS] WebSocket error");
    };

    ws.onclose = () => {
      wsRef.current = null;
      if (manualDisconnect.current) {
        setStatus("disconnected");
        return;
      }
      setStatus("reconnecting");
      const delay = Math.min(backoffRef.current, MAX_BACKOFF_MS);
      backoffRef.current = Math.min(backoffRef.current * 2, MAX_BACKOFF_MS);
      console.info(`[useClusterWS] Reconnecting in ${delay}ms`);
      timerRef.current = setTimeout(() => connect(), delay);
    };
  }, [token, cleanup]); // eslint-disable-line react-hooks/exhaustive-deps

  const disconnect = useCallback(() => {
    manualDisconnect.current = true;
    cleanup();
    setStatus("disconnected");
  }, [cleanup]);

  const reconnect = useCallback(() => {
    backoffRef.current = BASE_BACKOFF_MS;
    connect();
  }, [connect]);

  // Connect/disconnect when token changes
  useEffect(() => {
    if (token) {
      connect();
    } else {
      manualDisconnect.current = true;
      cleanup();
      setStatus("disconnected");
    }
    return cleanup;
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  return { snapshot, status, lastUpdated, disconnect, reconnect };
}

