// Package membershipkernel contains the domain-neutral invariants shared by
// buyer and seller membership control planes. It deliberately has no database,
// network, or provider dependencies.
package membershipkernel

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SubjectType identifies the domain that owns a membership.
type SubjectType string

const (
	SubjectTypeBuyer  SubjectType = "BUYER"
	SubjectTypeSeller SubjectType = "SELLER"
)

var (
	ErrInvalidSubjectType = errors.New("invalid membership subject type")
	ErrInvalidSubjectID   = errors.New("invalid membership subject id")
)

var subjectIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

// SubjectRef is the explicit cross-service identity used by membership
// commands and events. A buyer subject is a buyer_id; a seller subject is a
// store_id. An account_id is intentionally not represented here.
type SubjectRef struct {
	SubjectType SubjectType
	SubjectID   string
}

// NewSubjectRef validates and constructs an explicit domain subject.
func NewSubjectRef(subjectType SubjectType, subjectID string) (SubjectRef, error) {
	ref := SubjectRef{SubjectType: subjectType, SubjectID: subjectID}
	if err := ref.Validate(); err != nil {
		return SubjectRef{}, err
	}
	return ref, nil
}

// Validate enforces the current identity contract. IDs are decimal TSIDs at
// the service boundary, encoded as strings to avoid JSON/proto number loss.
func (s SubjectRef) Validate() error {
	switch s.SubjectType {
	case SubjectTypeBuyer, SubjectTypeSeller:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidSubjectType, s.SubjectType)
	}
	if s.SubjectID == "" || strings.TrimSpace(s.SubjectID) != s.SubjectID || !subjectIDPattern.MatchString(s.SubjectID) {
		return fmt.Errorf("%w: %q", ErrInvalidSubjectID, s.SubjectID)
	}
	return nil
}

// DomainKey is a stable, unambiguous key for logs, idempotency, and local
// projections. The type prefix prevents buyer and seller numeric IDs from
// colliding.
func (s SubjectRef) DomainKey() string {
	return string(s.SubjectType) + ":" + s.SubjectID
}
