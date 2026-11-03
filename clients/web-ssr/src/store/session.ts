import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface SessionPlayer {
  id: string;
  alias: string;
  role: 'player' | 'admin';
  token: string;
}

/** Cookie que leen las páginas renderizadas en el servidor (SSR) para llamar al gateway. */
export const SESSION_COOKIE = 'ja_token';

function writeCookie(token: string | null): void {
  if (typeof document === 'undefined') return;
  document.cookie = token
    ? `${SESSION_COOKIE}=${encodeURIComponent(token)}; Path=/; Max-Age=43200; SameSite=Lax`
    : `${SESSION_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
}

interface SessionState {
  player: SessionPlayer | null;
  setPlayer: (player: SessionPlayer) => void;
  clear: () => void;
}

/**
 * Sesión del jugador (JWT emitido por auth-identity-ms). Se guarda en localStorage para
 * sobrevivir recargas y en una cookie para que el servidor de Next pueda renderizar las
 * páginas protegidas. El token siempre lo valida el gateway; aquí no se confía en nada.
 */
export const useSession = create<SessionState>()(
  persist(
    (set) => ({
      player: null,
      setPlayer: (player) => {
        writeCookie(player.token);
        set({ player });
      },
      clear: () => {
        writeCookie(null);
        set({ player: null });
      },
    }),
    {
      name: 'jokerarena.session',
      onRehydrateStorage: () => (state) => writeCookie(state?.player?.token ?? null),
    },
  ),
);
