// Package contract4competiostest provides reusable fail-closed conformance
// suites for generic execution providers, lifecycle sinks, scoped-operation
// token authorities, and staged source-artifact providers.
//
// The suites are split by boundary so consumers can implement one port at a
// time: provider_conformance.go, event_conformance.go, grant_conformance.go,
// and source_conformance.go. Canonical Chess-shaped, unrelated Bidding
// Tic-Tac-Toe, three-slot tied, and deliberately unsafe implementations live in
// the package tests.
package contract4competiostest
