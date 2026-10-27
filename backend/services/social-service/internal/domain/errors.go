package domain

import (
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// ErrVersionConflict is what a delegated write gets when the post moved between
// the read that produced expectedVersion and the write itself (56 §5.4 / §6.6.4).
//
// Affected-rows = 0 is a conflict outright, never "read it again and overwrite":
// the version column exists precisely so the seller's concurrent edit cannot be
// lost silently. The client reloads, merges, and retries with a fresh
// expectedVersion and a fresh Idempotency-Key.
var ErrVersionConflict = pkgerrors.NewBusiness(
	bizcode.DelegationResourceVersionConflict, "資料已被更新，請重新載入後再試")

// ErrResourceNotOwned is Q4 (§7.1.2): the resource exists but belongs to a
// different store than the delegation targets.
//
// It is a separate judgement from the capability check because GetPost has no
// seller filter of its own — that is deliberate, four governance paths depend on
// reading posts they do not own — so nothing below this layer will notice a
// cross-store read. Without it a delegated reader retrieves any seller's post
// detail and, because a successful read writes no audit row, leaves no trace.
var ErrResourceNotOwned = pkgerrors.NewBusiness(
	bizcode.DelegationResourceNotOwned, "資源不屬於本次代行的店家")
