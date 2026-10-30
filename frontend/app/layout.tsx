import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'Webhook Delivery Engine - Console',
  description: 'High-throughput, reliable webhook delivery with exponential backoff & HMAC signatures',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-950 text-slate-100 antialiased">{children}</body>
    </html>
  )
}
