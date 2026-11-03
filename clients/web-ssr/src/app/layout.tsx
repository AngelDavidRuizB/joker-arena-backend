import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import '@fontsource/orbitron/500.css';
import '@fontsource/orbitron/700.css';
import '@fontsource/orbitron/900.css';
import '@fontsource/rajdhani/400.css';
import '@fontsource/rajdhani/500.css';
import '@fontsource/rajdhani/600.css';
import '@fontsource/rajdhani/700.css';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/700.css';
import './globals.css';
import { Layout } from '../components/Layout';

export const metadata: Metadata = {
  title: 'JokerArena · Multiverse Poker Engine',
  description: 'Póker con Jokers en tiempo real — Prototipo 2',
  icons: { icon: '/favicon.svg' },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="es">
      <body>
        <Layout>{children}</Layout>
      </body>
    </html>
  );
}
