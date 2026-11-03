interface LogoProps {
  size?: 'sm' | 'lg';
}

export function Logo({ size = 'sm' }: LogoProps) {
  const big = size === 'lg';
  return (
    <div className="flex items-center gap-3" aria-label="JokerArena">
      <svg
        viewBox="0 0 64 64"
        className={big ? 'h-16 w-16 animate-float' : 'h-9 w-9'}
        role="img"
        aria-hidden="true"
      >
        <defs>
          <linearGradient id="logo-g" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#00f0ff" />
            <stop offset="1" stopColor="#ff2bd6" />
          </linearGradient>
        </defs>
        <path
          d="M32 6 L40 23 L58 26 L45 39 L48 58 L32 49 L16 58 L19 39 L6 26 L24 23 Z"
          fill="rgba(10,7,32,.8)"
          stroke="url(#logo-g)"
          strokeWidth="3.5"
          strokeLinejoin="round"
        />
        <circle cx="32" cy="32" r="5" fill="url(#logo-g)" />
      </svg>
      <div className="leading-none">
        <div
          className={`font-display font-black tracking-[0.18em] ${
            big ? 'text-4xl sm:text-5xl' : 'text-lg'
          } bg-gradient-to-r from-neon-cyan via-white to-neon-magenta bg-clip-text text-transparent`}
        >
          JOKERARENA
        </div>
        <div className={`label-caps ${big ? 'mt-2 !text-xs' : 'mt-1'}`}>Multiverse Poker Engine</div>
      </div>
    </div>
  );
}
