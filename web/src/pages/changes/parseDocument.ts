// Minimal YAML 1.2 subset so the changes page can accept operator YAML without
// adding an npm parser (web/package.json is owned by other PRs).

export class DocumentParseError extends Error {
  readonly line?: number

  constructor(message: string, line?: number) {
    super(line ? `${message} (line ${line})` : message)
    this.name = 'DocumentParseError'
    this.line = line
  }
}

type YLine = {
  indent: number
  raw: string
  lineno: number
}

export function parseYamlOrJson(text: string): unknown {
  const src = text.replace(/^\uFEFF/, '')
  if (src.trim() === '') {
    throw new DocumentParseError('document is empty')
  }
  try {
    return JSON.parse(src)
  } catch {
    return parseYamlDocument(src)
  }
}

function parseYamlDocument(text: string): unknown {
  const lines = tokenize(text)
  if (lines.length === 0) {
    throw new DocumentParseError('document is empty')
  }
  const [value, next] = parseNode(lines, 0, lines[0].indent)
  if (next < lines.length) {
    throw new DocumentParseError('unexpected content', lines[next].lineno)
  }
  return value
}

function tokenize(text: string): YLine[] {
  const out: YLine[] = []
  const rows = text.split(/\r?\n/)
  for (let i = 0; i < rows.length; i++) {
    const expanded = rows[i].replace(/\t/g, '  ')
    const trimmedRight = stripComment(expanded)
    if (trimmedRight.trim() === '') {
      continue
    }
    const t = trimmedRight.trim()
    if (t === '---' || t === '...') {
      continue
    }
    const indent = trimmedRight.length - trimmedRight.trimStart().length
    out.push({ indent, raw: trimmedRight.trimStart(), lineno: i + 1 })
  }
  return out
}

function stripComment(line: string): string {
  let inSingle = false
  let inDouble = false
  for (let i = 0; i < line.length; i++) {
    const c = line[i]
    if (c === "'" && !inDouble) {
      inSingle = !inSingle
    } else if (c === '"' && !inSingle && (i === 0 || line[i - 1] !== '\\')) {
      inDouble = !inDouble
    } else if (c === '#' && !inSingle && !inDouble) {
      return line.slice(0, i).trimEnd()
    }
  }
  return line
}

function isSeqItem(raw: string): boolean {
  return raw === '-' || raw.startsWith('- ')
}

function parseNode(lines: YLine[], i: number, indent: number): [unknown, number] {
  if (i >= lines.length) {
    return [null, i]
  }
  const line = lines[i]
  if (line.indent < indent) {
    return [null, i]
  }
  if (isSeqItem(line.raw)) {
    return parseSeq(lines, i, line.indent)
  }
  if (splitKV(line.raw)) {
    return parseMap(lines, i, line.indent)
  }
  return [parseScalarOrFlow(line.raw), i + 1]
}

function parseMap(lines: YLine[], i: number, indent: number): [Record<string, unknown>, number] {
  const obj: Record<string, unknown> = {}
  while (i < lines.length) {
    const line = lines[i]
    if (line.indent < indent) {
      break
    }
    if (line.indent > indent) {
      throw new DocumentParseError('unexpected indent', line.lineno)
    }
    if (isSeqItem(line.raw)) {
      break
    }
    const kv = splitKV(line.raw)
    if (!kv) {
      throw new DocumentParseError('expected mapping entry', line.lineno)
    }
    const [key, rest] = kv
    if (rest === '') {
      if (i + 1 < lines.length && lines[i + 1].indent > indent) {
        const [child, n] = parseNode(lines, i + 1, lines[i + 1].indent)
        obj[key] = child
        i = n
      } else {
        obj[key] = null
        i += 1
      }
    } else {
      obj[key] = parseScalarOrFlow(rest)
      i += 1
    }
  }
  return [obj, i]
}

function parseSeq(lines: YLine[], i: number, indent: number): [unknown[], number] {
  const arr: unknown[] = []
  while (i < lines.length) {
    const line = lines[i]
    if (line.indent < indent) {
      break
    }
    if (line.indent > indent) {
      throw new DocumentParseError('unexpected indent', line.lineno)
    }
    if (!isSeqItem(line.raw)) {
      break
    }
    const rest = line.raw === '-' ? '' : line.raw.slice(2)
    if (rest === '') {
      if (i + 1 < lines.length && lines[i + 1].indent > indent) {
        const [child, n] = parseNode(lines, i + 1, lines[i + 1].indent)
        arr.push(child)
        i = n
      } else {
        arr.push(null)
        i += 1
      }
      continue
    }
    if (splitKV(rest)) {
      const synth: YLine = { indent: indent + 2, raw: rest, lineno: line.lineno }
      const tmp = [synth, ...lines.slice(i + 1)]
      const [obj, n] = parseMap(tmp, 0, synth.indent)
      arr.push(obj)
      i += n
      continue
    }
    arr.push(parseScalarOrFlow(rest))
    i += 1
  }
  return [arr, i]
}

function splitKV(raw: string): [string, string] | null {
  let inSingle = false
  let inDouble = false
  for (let i = 0; i < raw.length; i++) {
    const c = raw[i]
    if (c === "'" && !inDouble) {
      inSingle = !inSingle
    } else if (c === '"' && !inSingle && (i === 0 || raw[i - 1] !== '\\')) {
      inDouble = !inDouble
    } else if (c === ':' && !inSingle && !inDouble) {
      const next = raw[i + 1]
      if (next !== undefined && next !== ' ' && next !== '\t') {
        continue
      }
      const key = unquoteKey(raw.slice(0, i).trim())
      const rest = raw.slice(i + 1).trim()
      if (key === '') {
        return null
      }
      return [key, rest]
    }
  }
  return null
}

function unquoteKey(key: string): string {
  if (
    (key.startsWith('"') && key.endsWith('"') && key.length >= 2) ||
    (key.startsWith("'") && key.endsWith("'") && key.length >= 2)
  ) {
    const v = parseScalarOrFlow(key)
    return typeof v === 'string' ? v : key
  }
  return key
}

function parseScalarOrFlow(raw: string): unknown {
  const t = raw.trim()
  if (t.startsWith('{') && t.endsWith('}')) {
    return parseFlowMap(t)
  }
  if (t.startsWith('[') && t.endsWith(']')) {
    return parseFlowSeq(t)
  }
  return parseScalar(t)
}

function parseScalar(t: string): unknown {
  if (t.length >= 2 && ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith("'") && t.endsWith("'")))) {
    return decodeQuoted(t)
  }
  const lower = t.toLowerCase()
  if (lower === 'true') {
    return true
  }
  if (lower === 'false') {
    return false
  }
  if (lower === 'null' || lower === '~' || t === '') {
    return null
  }
  if (/^-?(0|[1-9]\d*)$/.test(t)) {
    const n = Number(t)
    if (Number.isSafeInteger(n)) {
      return n
    }
  }
  if (/^-?(0|[1-9]\d*)\.\d+$/.test(t)) {
    return Number(t)
  }
  return t
}

function decodeQuoted(t: string): string {
  const q = t[0]
  const inner = t.slice(1, -1)
  if (q === "'") {
    return inner.replace(/''/g, "'")
  }
  return inner.replace(/\\([\\nrt"])/g, (_m, ch: string) => {
    switch (ch) {
      case 'n':
        return '\n'
      case 'r':
        return '\r'
      case 't':
        return '\t'
      default:
        return ch
    }
  })
}

function parseFlowSeq(t: string): unknown[] {
  const inner = t.slice(1, -1).trim()
  if (inner === '') {
    return []
  }
  return splitTopLevel(inner, ',').map((part) => parseScalarOrFlow(part.trim()))
}

function parseFlowMap(t: string): Record<string, unknown> {
  const inner = t.slice(1, -1).trim()
  const obj: Record<string, unknown> = {}
  if (inner === '') {
    return obj
  }
  for (const part of splitTopLevel(inner, ',')) {
    const kv = splitKV(part.trim())
    if (!kv) {
      throw new DocumentParseError('invalid flow mapping')
    }
    obj[kv[0]] = parseScalarOrFlow(kv[1])
  }
  return obj
}

function splitTopLevel(s: string, sep: string): string[] {
  const parts: string[] = []
  let start = 0
  let depthBrace = 0
  let depthBracket = 0
  let inSingle = false
  let inDouble = false
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if (c === "'" && !inDouble) {
      inSingle = !inSingle
    } else if (c === '"' && !inSingle && (i === 0 || s[i - 1] !== '\\')) {
      inDouble = !inDouble
    } else if (!inSingle && !inDouble) {
      if (c === '{') {
        depthBrace += 1
      } else if (c === '}') {
        depthBrace -= 1
      } else if (c === '[') {
        depthBracket += 1
      } else if (c === ']') {
        depthBracket -= 1
      } else if (c === sep && depthBrace === 0 && depthBracket === 0) {
        parts.push(s.slice(start, i))
        start = i + 1
      }
    }
  }
  parts.push(s.slice(start))
  return parts
}
