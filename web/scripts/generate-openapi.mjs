#!/usr/bin/env node
// Writes web/src/api/openapi.d.ts from api/openapi/v1.json.
// Local emitter: openapi-typescript@7 cannot load TypeScript 7's compiler API.
import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const webRoot = resolve(here, '..')
const specPath = resolve(webRoot, '..', 'api', 'openapi', 'v1.json')
const destPath = resolve(webRoot, 'src', 'api', 'openapi.d.ts')
const check = process.argv.includes('--check')

const HTTP_METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace']
const PARAM_INS = ['query', 'header', 'path', 'cookie']

const spec = JSON.parse(readFileSync(specPath, 'utf8'))

function ident(name) {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(name) ? name : JSON.stringify(name)
}

function emitSchema(schema, indent = '') {
  if (!schema || typeof schema !== 'object') {
    return 'unknown'
  }
  if (schema.$ref) {
    const m = /^#\/components\/schemas\/(.+)$/.exec(schema.$ref)
    if (m) {
      return `components["schemas"][${JSON.stringify(m[1])}]`
    }
    return 'unknown'
  }
  if (Array.isArray(schema.enum) && schema.enum.length > 0) {
    return schema.enum.map((v) => JSON.stringify(v)).join(' | ')
  }
  if (schema.const !== undefined) {
    return JSON.stringify(schema.const)
  }
  const types = Array.isArray(schema.type) ? schema.type : schema.type ? [schema.type] : []
  if (types.includes('null') && types.length > 1) {
    const inner = emitSchema({ ...schema, type: types.filter((t) => t !== 'null') }, indent)
    return `${inner} | null`
  }
  if (types.length === 1 && types[0] === 'array') {
    return `(${emitSchema(schema.items ?? {}, indent)})[]`
  }
  if (types.length === 1 && (types[0] === 'integer' || types[0] === 'number')) {
    return 'number'
  }
  if (types.length === 1 && types[0] === 'boolean') {
    return 'boolean'
  }
  if (types.length === 1 && types[0] === 'string') {
    return 'string'
  }
  if (schema.properties || schema.additionalProperties || types.includes('object') || types.length === 0) {
    const required = new Set(schema.required ?? [])
    const props = schema.properties ?? {}
    const keys = Object.keys(props)
    const pad = indent + '  '
    const fields = keys.map((key) => {
      const opt = required.has(key) ? '' : '?'
      return `${pad}${ident(key)}${opt}: ${emitSchema(props[key], pad)};`
    })
    if (schema.additionalProperties && typeof schema.additionalProperties === 'object') {
      fields.push(`${pad}[key: string]: ${emitSchema(schema.additionalProperties, pad)};`)
    } else if (keys.length === 0) {
      return '{ [key: string]: unknown }'
    }
    return `{\n${fields.join('\n')}\n${indent}}`
  }
  return 'unknown'
}

function emitParamGroup(params, where) {
  const group = params.filter((p) => p.in === where)
  if (group.length === 0) {
    return 'never'
  }
  const required = group.some((p) => p.required)
  const fields = group.map((p) => {
    const opt = p.required ? '' : '?'
    return `    ${ident(p.name)}${opt}: ${emitSchema(p.schema ?? {}, '    ')};`
  })
  return { required, text: `{\n${fields.join('\n')}\n  }` }
}

function emitParameters(params) {
  if (!params || params.length === 0) {
    return `    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };`
  }
  const lines = PARAM_INS.map((where) => {
    const g = emitParamGroup(params, where)
    if (g === 'never') {
      return `      ${where}?: never;`
    }
    const opt = g.required ? '' : '?'
    return `      ${where}${opt}: ${g.text};`
  })
  return `    parameters: {\n${lines.join('\n')}\n    };`
}

function emitContent(content, indent) {
  if (!content || typeof content !== 'object') {
    return 'never'
  }
  const types = Object.keys(content)
  if (types.length === 0) {
    return 'never'
  }
  const pad = indent + '  '
  const fields = types.map((ct) => {
    const schema = content[ct]?.schema ?? {}
    return `${pad}${JSON.stringify(ct)}: ${emitSchema(schema, pad)};`
  })
  return `{\n${fields.join('\n')}\n${indent}}`
}

function emitResponses(responses) {
  const codes = Object.keys(responses ?? {})
  const fields = codes.map((code) => {
    const resp = responses[code]
    const key = code === 'default' ? 'default' : /^\d+$/.test(code) ? code : JSON.stringify(code)
    if (!resp?.content) {
      return `      ${key}: {
        headers: { [name: string]: unknown };
        content?: never;
      };`
    }
    return `      ${key}: {
        headers: { [name: string]: unknown };
        content: ${emitContent(resp.content, '        ')};
      };`
  })
  return `    responses: {\n${fields.join('\n')}\n    };`
}

function emitRequestBody(body) {
  if (!body) {
    return '    requestBody?: never;'
  }
  const opt = body.required === false ? '?' : ''
  return `    requestBody${opt}: {
      content: ${emitContent(body.content, '      ')};
    };`
}

function emitOperation(op) {
  return `{
${emitParameters(op.parameters)}
${emitRequestBody(op.requestBody)}
${emitResponses(op.responses)}
  }`
}

function emitPaths(paths) {
  const blocks = Object.entries(paths).map(([path, item]) => {
    const methods = HTTP_METHODS.map((method) => {
      const op = item[method]
      if (!op) {
        return `    ${method}?: never;`
      }
      const id = op.operationId || `${method}_${path.replace(/[^A-Za-z0-9]+/g, '_')}`
      return `    ${method}: operations[${JSON.stringify(id)}];`
    })
    return `  ${JSON.stringify(path)}: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
${methods.join('\n')}
  };`
  })
  return `export interface paths {\n${blocks.join('\n')}\n}`
}

function emitOperations(paths) {
  const ops = []
  for (const [path, item] of Object.entries(paths)) {
    for (const method of HTTP_METHODS) {
      const op = item[method]
      if (!op) {
        continue
      }
      const id = op.operationId || `${method}_${path.replace(/[^A-Za-z0-9]+/g, '_')}`
      ops.push(`  ${JSON.stringify(id)}: ${emitOperation(op)};`)
    }
  }
  return `export interface operations {\n${ops.join('\n')}\n}`
}

function emitSchemas(schemas) {
  const fields = Object.entries(schemas).map(
    ([name, schema]) => `    ${ident(name)}: ${emitSchema(schema, '    ')};`,
  )
  return `export interface components {
  schemas: {
${fields.join('\n')}
  };
  responses: never;
  parameters: never;
  requestBodies: never;
  headers: never;
  pathItems: never;
}`
}

const generated = `/**
 * This file was generated by web/scripts/generate-openapi.mjs from api/openapi/v1.json.
 * Do not edit.
 */

${emitPaths(spec.paths ?? {})}

${emitSchemas(spec.components?.schemas ?? {})}

export type $defs = Record<string, never>;

${emitOperations(spec.paths ?? {})}
`

if (check) {
  let existing = ''
  try {
    existing = readFileSync(destPath, 'utf8')
  } catch {
    console.error(`web-generate: missing ${destPath}; run make web-generate`)
    process.exit(1)
  }
  if (existing !== generated) {
    console.error('web-generate: web/src/api/openapi.d.ts is stale; run make web-generate')
    process.exit(1)
  }
  process.exit(0)
}

writeFileSync(destPath, generated)
console.log(`wrote ${destPath}`)
