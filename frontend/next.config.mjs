/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
  // Required for Docker standalone image
  output: 'standalone',
  experimental: {
    // Needed for reactflow SSR
  },
  webpack: (config) => {
    config.resolve.alias = {
      ...config.resolve.alias,
    }
    return config
  },
}

export default nextConfig
