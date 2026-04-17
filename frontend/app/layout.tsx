import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'InfraCanvas — Infrastructure at a glance',
  description: 'Visual infrastructure discovery and monitoring for your VMs and Kubernetes clusters.',
  icons: {
    icon: "data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⬡</text></svg>",
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="bg-background text-text-primary antialiased min-h-screen">
        {children}
      </body>
    </html>
  )
}
