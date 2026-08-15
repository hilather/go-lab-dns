// Package effects executes a chaos.ActionPlan on the DNS data plane.
//
// Decide remains side-effect-free except for explanation. Sleep, response
// mutation, transport hints, cache/upstream hooks, and pressure run here.
// ADR 0007 malformed-wire effects are not implemented.
package effects
