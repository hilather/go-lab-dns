export function YamlJsonEditor({
  id = 'yaml-json-editor',
  label = 'YAML or JSON',
  value,
  onChange,
  parseError,
  disabled = false,
}: {
  id?: string
  label?: string
  value: string
  onChange: (next: string) => void
  parseError?: string
  disabled?: boolean
}) {
  return (
    <div className="yaml-json-editor">
      <label htmlFor={id}>{label}</label>
      <textarea
        id={id}
        value={value}
        disabled={disabled}
        spellCheck={false}
        rows={18}
        onChange={(ev) => onChange(ev.target.value)}
      />
      {parseError ? (
        <p role="alert" className="problem">
          {parseError}
        </p>
      ) : null}
    </div>
  )
}
