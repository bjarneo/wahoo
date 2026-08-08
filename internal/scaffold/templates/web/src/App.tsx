const features = [
  ["SSR by default", "React renders at the edge of your request, with a safe client fallback."],
  ["Go at the core", "Typed handlers, graceful shutdown, structured logs, and boring deployment."],
  ["Realtime ready", "SSE and WebSocket endpoints are already wired into the generated service."],
];

export default function App() {
  return (
    <main className="shell">
      <nav className="nav">
        <span className="mark">W</span>
        <span className="wordmark">__APP_NAME__</span>
        <span className="nav-note">a calm place to ship</span>
        <a className="nav-link" href="/dashboard">Dashboard <span>-&gt;</span></a>
      </nav>
      <section className="hero">
        <p className="eyebrow">GO / REACT / TAILWIND</p>
        <h1>Ship the useful part.<br /><em>Keep the rest boring.</em></h1>
        <p className="lede">A production-shaped SaaS foundation with a Go runtime, React SSR, and the escape hatches serious products need.</p>
        <div className="actions">
          <a className="button button-primary" href="/dashboard">Open dashboard <span>-&gt;</span></a>
          <a className="button button-quiet" href="https://github.com/bjarneo/wahoo">Read the guide</a>
        </div>
      </section>
      <section className="feature-grid" aria-label="Framework features">
        {features.map(([title, copy], index) => (
          <article className="feature" key={title}>
            <span className="feature-number">0{index + 1}</span>
            <h2>{title}</h2>
            <p>{copy}</p>
          </article>
        ))}
      </section>
      <footer className="footer"><span>Ready when you are.</span><span>health: <strong>ok</strong></span></footer>
    </main>
  );
}
