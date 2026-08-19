export function ProblemAlert({
  code,
  detail,
  error,
}: {
  code?: string
  detail?: string
  error?: { code?: string; detail?: string; message?: string } | null
}) {
  const c = code || error?.code || ''
  const d = detail || error?.detail || error?.message || ''
  if (c === '' && d === '') {
    return null
  }
  const text = c !== '' && d !== '' ? `${c}: ${d}` : c || d
  return (
    <p role="alert" className="problem">
      {text}
    </p>
  )
}
