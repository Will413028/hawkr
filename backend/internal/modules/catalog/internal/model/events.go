package model

import (
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

type ProductWritten struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	ProductID  uuid.UUID
	StoreID    int64
	Title      string
	Version    int64
	Actor      *delegatedactor.Actor
}

type ProductCreated struct{ ProductWritten }

type ProductUpdated struct{ ProductWritten }

type ProductPublished struct{ ProductWritten }

type ProductUnpublished struct{ ProductWritten }

type ProductPublishRepeated struct{ ProductWritten }

type ProductUnpublishRepeated struct{ ProductWritten }

type ProductsUnpublished struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	StoreID    int64
	Items      []UnpublishedItem
	Actor      *delegatedactor.Actor
}

type UnpublishedItem struct {
	ProductID     uuid.UUID
	Title         string
	VersionBefore int64
	VersionAfter  int64
	Changed       bool
}

type VersionedProduct struct {
	ProductID uuid.UUID
	Version   int64
}

type BatchVersionConflictError struct{ ProductIDs []uuid.UUID }

func (e *BatchVersionConflictError) Error() string { return "catalog: batch product version conflict" }

func (e *BatchVersionConflictError) Is(target error) bool { return target == ErrVersionConflict }

type BatchNotPublishedError struct{ ProductIDs []uuid.UUID }

func (e *BatchNotPublishedError) Error() string { return "catalog: batch contains draft products" }

func (e *BatchNotPublishedError) Is(target error) bool { return target == ErrNotPublished }

func (e *BatchVersionConflictError) SortedIDs() []uuid.UUID {
	out := slices.Clone(e.ProductIDs)
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}
