/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: {
          950: '#05030f',
          900: '#0a0720',
          800: '#120d33',
          700: '#1c1550',
          600: '#2a2070',
        },
        neon: {
          cyan: '#00f0ff',
          magenta: '#ff2bd6',
          violet: '#8b5cf6',
          lime: '#39ff88',
          amber: '#ffb800',
          red: '#ff4d6d',
        },
      },
      fontFamily: {
        display: ['Orbitron', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        body: ['Rajdhani', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
      boxShadow: {
        'neon-cyan': '0 0 12px rgba(0,240,255,.55), 0 0 32px rgba(0,240,255,.25)',
        'neon-magenta': '0 0 12px rgba(255,43,214,.55), 0 0 32px rgba(255,43,214,.25)',
        'neon-violet': '0 0 12px rgba(139,92,246,.55), 0 0 32px rgba(139,92,246,.25)',
        'neon-lime': '0 0 12px rgba(57,255,136,.55), 0 0 32px rgba(57,255,136,.25)',
        'neon-amber': '0 0 12px rgba(255,184,0,.55), 0 0 32px rgba(255,184,0,.25)',
      },
      keyframes: {
        float: {
          '0%,100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-8px)' },
        },
        gridMove: {
          '0%': { backgroundPosition: '0 0' },
          '100%': { backgroundPosition: '0 64px' },
        },
        pulseGlow: {
          '0%,100%': { opacity: '.55' },
          '50%': { opacity: '1' },
        },
        flicker: {
          '0%,19%,21%,23%,25%,54%,56%,100%': { opacity: '1' },
          '20%,24%,55%': { opacity: '.55' },
        },
        shine: {
          '0%': { transform: 'translateX(-120%) skewX(-20deg)' },
          '100%': { transform: 'translateX(220%) skewX(-20deg)' },
        },
      },
      animation: {
        float: 'float 6s ease-in-out infinite',
        'grid-move': 'gridMove 6s linear infinite',
        'pulse-glow': 'pulseGlow 2.4s ease-in-out infinite',
        flicker: 'flicker 5s linear infinite',
        shine: 'shine 2.8s ease-in-out infinite',
      },
    },
  },
  plugins: [],
};
