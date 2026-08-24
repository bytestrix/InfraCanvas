import type { Metadata } from 'next'
import './globals.css'
import '@xterm/xterm/css/xterm.css'

export const metadata: Metadata = {
  title: 'InfraCanvas | Infrastructure at a glance',
  description: 'Real-time visual infrastructure discovery for VMs, containers, and Kubernetes.',
  icons: {
    icon: '/logo.png',
  },
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="dark">
      <body className="min-h-screen antialiased">
        {children}
      </body>
    </html>
  )
}
