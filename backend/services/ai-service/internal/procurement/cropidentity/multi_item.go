package cropidentity

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
)

var itemConjunctions = []string{"、", "和", "與", "跟", "及"}

const minItemPartRunes = 2

func splitItems(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	for _, sep := range itemConjunctions {
		idx := strings.Index(text, sep)
		if idx <= 0 {
			continue
		}
		left := strings.TrimSpace(text[:idx])
		right := strings.TrimSpace(text[idx+len(sep):])
		if utf8.RuneCountInString(left) < minItemPartRunes || utf8.RuneCountInString(right) < minItemPartRunes {
			continue
		}
		return append([]string{left}, splitItems(right)...)
	}
	return []string{text}
}

func (uc *DisambiguationUseCase) resolveMultiItem(ctx context.Context, itemText string, parts []string) (DisambiguationResult, error) {
	pool := make([]recall.Candidate, 0, len(parts)*2)
	seen := map[string]bool{}
	for _, part := range parts {
		q := understandQuery(part)
		if len(q.Variants) == 0 {
			continue
		}
		usable, err := uc.taxonomyCandidates(ctx, q)
		if err != nil {
			observability.RecordProcurementCropResolverV2(ctx, "error")
			return DisambiguationResult{}, err
		}
		for _, f := range usable {
			if seen[f.Code] {
				continue
			}
			seen[f.Code] = true
			pool = append(pool, f)
		}
	}
	if len(pool) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryDescribeStage(nil), nil
	}
	return uc.categoryCard(ctx, itemText, pool)
}
