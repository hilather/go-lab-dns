// Package docs embeds the operator documentation served by both control adapters.
package docs

import "embed"

// Operator contains the canonical documentation files in this directory.
// Keeping the embed alongside its sources avoids generated copies.
//
//go:embed 02-dns-semantics.md 03-chaos-engine.md
var Operator embed.FS
