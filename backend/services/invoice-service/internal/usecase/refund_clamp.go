package usecase

import "github.com/Will413028/hawkr/invoice-service/internal/domain"

func clampAllowanceToRemaining(doc domain.Document, lines []domain.DocumentLine, requested int64) (effective, req int64) {
	effective = requested

	if docRemaining := doc.TotalAmount - doc.AllowanceTotalAmount; docRemaining < effective {
		effective = docRemaining
	}

	_, base := domain.AllowanceBaseOf(allowanceSourceLines(lines))
	var lineRemaining int64
	for _, b := range base {
		lineRemaining += b
	}
	if lineRemaining < effective {
		effective = lineRemaining
	}

	if effective < 0 {
		effective = 0
	}
	return effective, requested
}
