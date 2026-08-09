const features = [
  ["SSR by default", "React renders before hydration, while the Go server retains a basic fallback if the worker is unavailable."],
  ["Go at the core", "Typed handlers, graceful shutdown, structured logs, and a deployment model that stays understandable."],
  ["Realtime ready", "Bounded SSE and WebSocket primitives are ready when the product adds authorization."],
];

export default function App() {
  return (
    <>
      <a className="skip-link" href="#main-content">Skip to main content</a>
      <main className="shell" id="main-content">
        <header className="site-header">
          <nav className="nav" aria-label="Primary navigation">
            <span className="mark" aria-hidden="true">W</span>
            <span className="wordmark">__APP_NAME__</span>
            <span className="nav-note">a calm place to ship</span>
            <a className="nav-link" href="/dashboard">Dashboard <span aria-hidden="true">-&gt;</span></a>
          </nav>
        </header>
        <section className="hero" aria-labelledby="hero-heading">
          <p className="eyebrow">GO / REACT / TAILWIND</p>
          <h1 id="hero-heading">Ship the useful part.<br /><em>Keep the rest boring.</em></h1>
          <p className="lede">A production-shaped SaaS foundation with a Go runtime, React SSR, and the room to build a serious product.</p>
          <div className="actions">
            <a className="button button-primary" href="/dashboard">Open dashboard <span aria-hidden="true">-&gt;</span></a>
            <a className="button button-quiet" href="https://github.com/bjarneo/wahoo">Read the guide</a>
          </div>
        </section>
        <section className="features" aria-labelledby="features-heading">
          <h2 className="sr-only" id="features-heading">Framework features</h2>
          <ul className="feature-grid">
            {features.map(([title, copy], index) => (
              <li className="feature" key={title}>
                <span className="feature-number">0{index + 1}</span>
                <h3>{title}</h3>
                <p>{copy}</p>
              </li>
            ))}
          </ul>
        </section>
        <footer className="footer">
          <span>Ready when you are.</span>
          <span className="health"><span aria-hidden="true" className="health-dot" />System status: <strong>operational</strong></span>
        </footer>
      </main>
    </>
  );
}
