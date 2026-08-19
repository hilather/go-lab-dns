// PR-13 owns flush mutations; keep the control visible but inert so the page ships a slot.

export function FlushPanel() {
  return (
    <section>
      <h2>Flush</h2>
      <p>Requires dns.admin. Flush does not change desired state.</p>
      <label>
        <input type="checkbox" defaultChecked disabled /> Flush all entries
      </label>
      <p>
        <button type="button" disabled>
          Flush cache
        </button>
      </p>
    </section>
  )
}
