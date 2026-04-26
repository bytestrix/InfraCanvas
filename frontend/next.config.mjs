/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export so the dashboard can be embedded into the
  // `infracanvas serve` Go binary via go:embed.
  output: 'export',
  reactStrictMode: false,
  // next/image's default loader needs a server — disable for export.
  images: { unoptimized: true },
  // Trailing slashes give us per-route index.html files, which an embedded
  // http.FileServer can serve without a custom handler.
  trailingSlash: true,
}

export default nextConfig
