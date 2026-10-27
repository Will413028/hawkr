// Package event contains domain event definitions.
//
// Domain events represent significant occurrences in the domain that other parts
// of the system may be interested in. They are collected by entities during
// state transitions and dispatched after transaction commit.
//
// Each event should implement:
//   - EventType() string - returns a unique identifier for the event type
//   - Version() int - returns the event version for progressive migration
//
// This package is currently a placeholder. Add domain events as needed.
package event
