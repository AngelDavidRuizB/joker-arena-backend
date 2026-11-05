import { useState, useCallback } from "react";
import { LOGIN_URL } from "../config";

interface AuthState {
  token: string | null;
  alias: string | null;
  role: string | null;
  error: string | null;
  loading: boolean;
}

interface UseGatewayReturn extends AuthState {
  login: (alias: string, password: string) => Promise<void>;
  loginDemo: () => void;
  logout: () => void;
  isAdmin: boolean;
}

/**
 * Manages authentication against the API Gateway.
 * Stores the JWT in memory (not localStorage) so Tauri's renderer process
 * never persists credentials to disk.
 */
export function useGateway(): UseGatewayReturn {
  const [state, setState] = useState<AuthState>({
    token: null,
    alias: null,
    role: null,
    error: null,
    loading: false,
  });

  const login = useCallback(async (alias: string, password: string) => {
    setState((s) => ({ ...s, loading: true, error: null }));
    try {
      const res = await fetch(LOGIN_URL, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ alias, password }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message ?? `HTTP ${res.status}`);
      }

      const data = await res.json();
      // JWT payload (base64url decode the second segment)
      const payload = JSON.parse(atob(data.token.split(".")[1]));

      setState({
        token: data.token,
        alias: payload.alias ?? alias,
        role: payload.role ?? null,
        error: null,
        loading: false,
      });
    } catch (err) {
      setState((s) => ({
        ...s,
        loading: false,
        error: err instanceof Error ? err.message : "Error desconocido",
      }));
    }
  }, []);

  const logout = useCallback(() => {
    setState({ token: null, alias: null, role: null, error: null, loading: false });
  }, []);

  const loginDemo = useCallback(() => {
    setState({
      token: "demo-jwt-token",
      alias: "admin-demo",
      role: "admin",
      error: null,
      loading: false,
    });
  }, []);

  return { ...state, login, loginDemo, logout, isAdmin: state.role === "admin" };
}

