// Package chaos implements policy selectors, deterministic decisions, effects, and budgets.
//
// CHA-001 lands the policy index, hash-v1, gates, budgets, simulation, and
// the SIGUSR1 emergency path. Packet effects (delay/drop/RCODE) execute in CHA-002.
package chaos
