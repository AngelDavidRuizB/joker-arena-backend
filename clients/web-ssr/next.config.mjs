/** @type {import('next').NextConfig} */
const GATEWAY = process.env.GATEWAY_INTERNAL_URL ?? 'http://localhost:8080';

const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  poweredByHeader: false,
  // Las llamadas REST del navegador van al mismo origen (/api/...) y Next las reenvía al
  // gateway: así el navegador nunca habla con un microservicio directamente y no hay CORS.
  // El WebSocket del juego sí va directo al gateway (NEXT_PUBLIC_GATEWAY_WS_URL).
  async rewrites() {
    return [
      { source: '/api/:path((?!health).*)', destination: `${GATEWAY}/api/:path` },
      // Rutas de administración del gateway (/admin/*), sin chocar con la página /admin.
      { source: '/gw-admin/:path*', destination: `${GATEWAY}/admin/:path*` },
    ];
  },
};

export default nextConfig;
