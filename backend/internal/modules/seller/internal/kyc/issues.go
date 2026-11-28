package kyc

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func issueField(i int, name string) string { return fmt.Sprintf("Issues[%d].%s", i, name) }

func validateIssues(in []model.KYCIssue) ([]model.KYCIssue, error) {
	if len(in) == 0 {
		return nil, apperr.InvalidInputOn([]string{"Issues"}, "退回至少要指出一個問題")
	}
	fields, codes := model.KYCFieldNames(), model.KYCIssueCodeValues()
	out := make([]model.KYCIssue, len(in))
	for i, issue := range in {
		if !slices.Contains(fields, issue.Field) {
			return nil, apperr.InvalidInputOn([]string{issueField(i, "Field")}, "不認得的欄位")
		}
		if !slices.Contains(codes, issue.Code) {
			return nil, apperr.InvalidInputOn([]string{issueField(i, "Code")}, "不認得的原因碼")
		}
		message := strings.TrimSpace(issue.Message)
		if n := utf8.RuneCountInString(message); n < 1 || n > model.MaxKYCIssueMessage {
			return nil, apperr.InvalidInputOn([]string{issueField(i, "Message")}, "說明必填，最多 %d 字", model.MaxKYCIssueMessage)
		}
		asset := strings.TrimSpace(issue.DocumentAssetID)
		if asset != "" {
			id, err := uuid.Parse(asset)
			if err != nil {
				return nil, apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "文件 ID 格式不正確")
			}
			asset = id.String()
		}
		isDocument := model.IsDocumentField(issue.Field)
		switch {
		case asset != "" && !isDocument:
			return nil, apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "只有文件類的欄位可以指到一張文件")
		case isDocument && asset != "" && model.IssueAssetForbidden(issue.Code):
			return nil, apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "缺少文件指的是整個位置，不能指定某一張文件")
		case isDocument && asset == "" && model.IssueAssetRequired(issue.Code):
			return nil, apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "這個原因要指定是哪一張文件")
		}
		out[i] = model.KYCIssue{Field: issue.Field, DocumentAssetID: asset, Code: issue.Code, Message: message}
	}
	return out, nil
}

func checkIssuesAgainstApplication(issues []model.KYCIssue, sellerType model.KYCSellerType, docs []store.SellerKycDocument) error {
	for i, issue := range issues {
		if !slices.Contains(model.IssuableFields(sellerType), issue.Field) {
			return apperr.InvalidInputOn([]string{issueField(i, "Field")}, "這個欄位不適用於這份申請的賣家型態")
		}
		if issue.DocumentAssetID == "" {
			continue
		}
		onApplication := slices.ContainsFunc(docs, func(d store.SellerKycDocument) bool { return d.AssetID.String() == issue.DocumentAssetID })
		if !onApplication {
			return apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "指定的文件不是這份申請在這個欄位目前的文件")
		}
	}
	return nil
}

func insertIssues(ctx context.Context, q *store.Queries, reviewID uuid.UUID, issues []model.KYCIssue) error {
	for i, issue := range issues {
		params := store.InsertKYCReviewIssueParams{
			IssueID: uuid.NewV7(), ReviewID: reviewID, Position: int32(i), //nolint:gosec // 請求的問題數有上限
			Field: issue.Field, Code: string(issue.Code), Message: issue.Message,
		}
		if issue.DocumentAssetID != "" {
			id, err := uuid.Parse(issue.DocumentAssetID)
			if err != nil {
				return err
			}
			params.DocumentAssetID = &id
		}
		if err := q.InsertKYCReviewIssue(ctx, params); err != nil {
			return err
		}
	}
	return nil
}

func toIssue(row store.SellerKycReviewIssue) model.KYCIssue {
	return model.KYCIssue{Field: row.Field, DocumentAssetID: assetText(row.DocumentAssetID), Code: model.KYCIssueCode(row.Code), Message: row.Message}
}

func issuesByReview(rows []store.SellerKycReviewIssue) map[uuid.UUID][]model.KYCIssue {
	out := map[uuid.UUID][]model.KYCIssue{}
	for _, row := range rows {
		out[row.ReviewID] = append(out[row.ReviewID], toIssue(row))
	}
	return out
}

func pendingRejection(reviews []store.SellerKycReview) *store.SellerKycReview {
	i := len(reviews) - 1
	if i < 0 || reviews[i].Action != string(model.ReviewReject) {
		return nil
	}
	return &reviews[i]
}

func outstandingIssues(reviews []store.SellerKycReview, rows []store.SellerKycReviewIssue) []model.KYCIssue {
	out := []model.KYCIssue{}
	r := pendingRejection(reviews)
	if r == nil {
		return out
	}
	for _, row := range rows {
		if row.ReviewID == r.ReviewID && row.ResolvedAt == nil {
			out = append(out, toIssue(row))
		}
	}
	return out
}

func assetText(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func sortedAssetTexts(docs []store.SellerKycDocument) []string {
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.AssetID.String())
	}
	slices.Sort(out)
	return out
}

func issueResolved(field, assetID string, changed map[string]bool, before, after []store.SellerKycDocument) bool {
	switch {
	case !model.IsDocumentField(field):
		return changed[field]
	case assetID != "":
		return !slices.ContainsFunc(after, func(d store.SellerKycDocument) bool { return d.AssetID.String() == assetID })
	default:
		return !slices.Equal(sortedAssetTexts(before), sortedAssetTexts(after))
	}
}

func resolveIssues(ctx context.Context, q *store.Queries, applicationID uuid.UUID, now time.Time, diff draftDiff, docsBefore, docsAfter []store.SellerKycDocument) error {
	open, err := q.UnresolvedKYCReviewIssues(ctx, applicationID)
	if err != nil || len(open) == 0 {
		return err
	}
	var resolved []uuid.UUID
	for _, issue := range open {
		if issueResolved(issue.Field, assetText(issue.DocumentAssetID), diff.fields, docsBefore, docsAfter) {
			resolved = append(resolved, issue.IssueID)
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return q.ResolveKYCReviewIssues(ctx, store.ResolveKYCReviewIssuesParams{IssueIds: resolved, Now: now})
}
