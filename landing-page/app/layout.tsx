import type { Metadata } from 'next'
import { JetBrains_Mono, Inter } from 'next/font/google'
import { Analytics } from '@vercel/analytics/next'
import './globals.css'

const _inter = Inter({ subsets: ["latin"] });
const _jetbrainsMono = JetBrains_Mono({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: 'Skillbox — The Self-Hosted Execution Runtime for AI Agents',
  description: 'Secure, sandboxed execution of Python, Node.js, and Bash scripts via REST API. Self-hosted, open source, structured I/O. The missing piece for AI agent infrastructure.',
  openGraph: {
    title: 'Skillbox — The Self-Hosted Execution Runtime for AI Agents',
    description: 'Give your AI agents a sandbox. Self-hosted, open source, secure by default.',
    type: 'website',
  },
}

export const viewport = {
  themeColor: '#0a0a0a',
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en">
      <body className="font-sans antialiased bg-background text-foreground">
        {children}
        <Analytics />
      </body>
    </html>
  )
}
