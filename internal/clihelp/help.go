// Package clihelp is the operator-visible CLI help surface.
//
// The text is generated into api/cli/help.txt so release-diff can compare
// two refs without building binaries.
package clihelp

// RelPath is the generated CLI help artifact, relative to the module root.
const RelPath = "api/cli/help.txt"

// Text is the exact `labdns help` output, including the trailing newline.
const Text = `usage: labdns <command>
commands:
  serve --config PATH [--chaos-disable] [--dns-listen ADDR] [--management-listen ADDR|off] [--shutdown-timeout DUR] [--pid-file PATH]
  validate --config PATH
  canonicalize --config PATH [--format yaml|json]
  verify --config PATH --probes PATH
  query --name NAME [--type A] [--server HOST:PORT] [--transport udp|tcp]
  healthcheck --url URL
  chaos emergency-disable --pid-file PATH
  version
env: LABDNS_CHAOS_DISABLE=1 inhibits chaos at startup (YAML/API cannot relax it).
env: no environment variable raises chaos safety caps.
`
