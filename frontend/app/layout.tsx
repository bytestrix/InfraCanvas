import type { Metadata } from 'next'
import './globals.css'
import '@xterm/xterm/css/xterm.css'

export const metadata: Metadata = {
  title: 'InfraCanvas — Infrastructure at a glance',
  description: 'Real-time visual infrastructure discovery for VMs, containers, and Kubernetes.',
  icons: {
    icon: "data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⬡</text></svg>",
  },
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen antialiased" style={{ background: '#111110', color: '#F0EDE7' }}>
        {children}
      </body>
    </html>
  )
}
