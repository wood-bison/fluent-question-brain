import Link from 'next/link'

export default function Home() {
  return (
    <main style={{ fontFamily: 'system-ui', maxWidth: 760, margin: '0 auto', padding: 64 }}>
      <p style={{ letterSpacing: '0.14em', textTransform: 'uppercase' }}>Question Brain · G4</p>
      <h1>Editorial source for the question graph.</h1>
      <p>Draft and review bilingual cards in Payload. Publishing promotes one canonical revision to the Go API.</p>
      <Link href="/admin">Open the authoring studio →</Link>
    </main>
  )
}
