// Package compiler orchestrates compilation of canonical state into an
// immutable snapshot.
//
// Compile calls config.Normalize + config.Validate, then resolver.Compile,
// forwarder.Compile, and snapshot.CompileAccess. Chaos compile is a no-op
// until CHA-001. The returned Snapshot is immutable; callers must not
// mutate Canonical or the compiled indexes.
package compiler
