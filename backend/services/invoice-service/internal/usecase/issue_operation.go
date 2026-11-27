package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

type issueOperationSpec struct {
	OperationID         int64
	DocumentID          int64
	OriginalDocumentID  *int64
	IssueAttemptSeq     int32
	ProviderRequestNo   string
	RequestSnapshotHash string
	MaxAttempts         int32
	NextAttemptAt       time.Time
	StatutoryDeadlineAt *time.Time
	Now                 time.Time
}

func newIssueOperation(spec issueOperationSpec) (domain.NewOperation, error) {
	opType := codemeta.InvoiceOperationTypeIssue.Value
	if spec.OriginalDocumentID != nil {
		opType = codemeta.InvoiceOperationTypeReissue.Value
	}
	seq := spec.IssueAttemptSeq
	if seq < 1 {
		seq = 1
	}
	key, ok := domain.OperationKeyFor(opType, spec.DocumentID, 0, seq, 0)
	if !ok {
		return domain.NewOperation{}, fmt.Errorf("invoice: no operation_key rule for %s", opType)
	}
	return domain.NewOperation{
		ID:                  spec.OperationID,
		DocumentID:          spec.DocumentID,
		OperationType:       opType,
		OperationKey:        key,
		ProviderRequestNo:   spec.ProviderRequestNo,
		RequestSnapshotHash: spec.RequestSnapshotHash,
		MaxAttempts:         spec.MaxAttempts,
		NextAttemptAt:       spec.NextAttemptAt,
		StatutoryDeadlineAt: spec.StatutoryDeadlineAt,
		Now:                 spec.Now,
	}, nil
}

func createAndPromoteIssueOperation(
	ctx context.Context,
	ops domain.OperationRepository,
	spec issueOperationSpec,
	appendHistory func(ctx context.Context, operationID int64, from, to string) error,
) (int64, string, bool, error) {
	newOp, err := newIssueOperation(spec)
	if err != nil {
		return 0, "", false, err
	}
	op, inserted, err := ops.Create(ctx, newOp)
	if err != nil {
		return 0, "", false, err
	}
	if !inserted {
		op, err = ops.GetByKey(ctx, newOp.OperationKey)
		if err != nil {
			return 0, "", false, err
		}
		return op.ID, op.Status, false, nil
	}
	next := codemeta.InvoiceOperationStatusReady.Value
	promoted, err := ops.Promote(ctx, op.ID, next, spec.NextAttemptAt, spec.Now)
	if err != nil {
		return 0, "", false, err
	}
	if !promoted {
		return 0, "", false, fmt.Errorf("invoice: operation %d could not leave CREATED (rows=0)", op.ID)
	}
	if appendHistory != nil {
		if err := appendHistory(ctx, op.ID, codemeta.InvoiceOperationStatusCreated.Value, next); err != nil {
			return 0, "", false, err
		}
	}
	return op.ID, next, true, nil
}
