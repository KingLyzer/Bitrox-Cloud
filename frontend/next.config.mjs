/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    const proxyTarget = (process.env.API_PROXY_TARGET ?? "http://api:8080").replace(/\/+$/, "");
    return [
      {
        source: "/api/:path*",
        destination: `${proxyTarget}/api/:path*`
      },
      {
        source: "/health/:path*",
        destination: `${proxyTarget}/health/:path*`
      }
    ];
  }
};

export default nextConfig;
