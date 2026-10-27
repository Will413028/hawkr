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

// 退回的逐項問題（model.KYCIssue）：退回時寫入（kyc_review_issues），賣家在寫入申請時由同一個 transaction 比對新舊值標成
// 已處理（resolved_at），讀取時只列最近一次退回、賣家還沒處理的那些。審核紀錄不存欄位值，所以「改過沒有」只比寫入前後的列。

// issueField 是 Reject 的輸入錯誤指到的欄位路徑（模組輸入的欄位名，索引寫成 [i]）。
func issueField(i int, name string) string { return fmt.Sprintf("Issues[%d].%s", i, name) }

// validateIssues 檢查退回的問題清單與申請無關的部分：至少一項；欄位是 KYCFieldNames 的路徑、原因碼在值集合內、說明去掉前後空白
// 後 1 到 MaxKYCIssueMessage 字、非文件類的欄位不帶 asset ID；文件類的欄位依原因碼：UNREADABLE、EXPIRED、MISMATCH、INVALID 必須帶
// asset ID（問題出在某一張文件上），MISSING 不可以帶（缺的是整個位置），OTHER 兩者皆可。回傳正規化（去空白）後的清單。
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
		_, isDocument := model.KYCFieldDocumentSlot(issue.Field)
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

// checkIssuesAgainstApplication 檢查問題指到的是這份申請實際有的東西：欄位適用於申請的賣家型態；指到的文件是申請目前的文件，
// 而且在欄位對應的位置。
func checkIssuesAgainstApplication(issues []model.KYCIssue, sellerType model.KYCSellerType, docs []store.SellerKycDocument) error {
	for i, issue := range issues {
		if !slices.Contains(model.IssuableFields(sellerType), issue.Field) {
			return apperr.InvalidInputOn([]string{issueField(i, "Field")}, "這個欄位不適用於這份申請的賣家型態")
		}
		if issue.DocumentAssetID == "" {
			continue
		}
		slot, _ := model.KYCFieldDocumentSlot(issue.Field)
		onApplication := slices.ContainsFunc(docs, func(d store.SellerKycDocument) bool {
			return d.AssetID == issue.DocumentAssetID && d.Slot == string(slot)
		})
		if !onApplication {
			return apperr.InvalidInputOn([]string{issueField(i, "DocumentAssetID")}, "指定的文件不是這份申請在這個欄位目前的文件")
		}
	}
	return nil
}

// insertIssues 把退回的問題寫在審核紀錄底下，順序就是 position。
func insertIssues(ctx context.Context, q *store.Queries, reviewID uuid.UUID, issues []model.KYCIssue) error {
	for i, issue := range issues {
		params := store.InsertKYCReviewIssueParams{
			IssueID: uuid.NewV7(), ReviewID: reviewID, Position: int32(i), //nolint:gosec // 請求的問題數有上限
			Field: issue.Field, Code: string(issue.Code), Message: issue.Message,
		}
		if issue.DocumentAssetID != "" {
			params.DocumentAssetID = &issue.DocumentAssetID
		}
		if err := q.InsertKYCReviewIssue(ctx, params); err != nil {
			return err
		}
	}
	return nil
}

func toIssue(row store.SellerKycReviewIssue) model.KYCIssue {
	return model.KYCIssue{Field: row.Field, DocumentAssetID: model.Deref(row.DocumentAssetID), Code: model.KYCIssueCode(row.Code), Message: row.Message}
}

// issuesByReview 把整份申請的問題依審核紀錄分組（保持 position 的順序）。
func issuesByReview(rows []store.SellerKycReviewIssue) map[uuid.UUID][]model.KYCIssue {
	out := map[uuid.UUID][]model.KYCIssue{}
	for _, row := range rows {
		out[row.ReviewID] = append(out[row.ReviewID], toIssue(row))
	}
	return out
}

// pendingRejection 是賣家要處理的那次退回：審核紀錄（依時間排序）去掉 REBIND（管理員對已核准申請的動作）之後，最後一筆是 REJECT；
// 重新送出、核准、取消都會留下之後的紀錄，所以那之後就沒有了。
func pendingRejection(reviews []store.SellerKycReview) *store.SellerKycReview {
	i := len(reviews) - 1
	for i >= 0 && reviews[i].Action == string(model.ReviewRebind) {
		i--
	}
	if i < 0 || reviews[i].Action != string(model.ReviewReject) {
		return nil
	}
	return &reviews[i]
}

// outstandingIssues 是賣家還沒處理的問題：只算要處理的那次退回（pendingRejection）裡還沒標成已處理的。
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

// assetIDs 回傳某個位置的 asset ID（排序，順序不算）。
func assetIDs(docs []store.SellerKycDocument, slot string) []string {
	var out []string
	for _, d := range docs {
		if d.Slot == slot {
			out = append(out, d.AssetID)
		}
	}
	slices.Sort(out)
	return out
}

// issueResolved 回報一個問題在這次寫入之後是不是已處理：非文件類的欄位是值有變；文件類的欄位指到一張文件時是那張文件不在了
// （移除或替換），沒有指到特定一張時是那個位置的文件組成變了。
func issueResolved(field, assetID string, changed map[string]bool, before, after []store.SellerKycDocument) bool {
	slot, isDocument := model.KYCFieldDocumentSlot(field)
	switch {
	case !isDocument:
		return changed[field]
	case assetID != "":
		return !slices.ContainsFunc(after, func(d store.SellerKycDocument) bool { return d.AssetID == assetID })
	default:
		return !slices.Equal(assetIDs(before, string(slot)), assetIDs(after, string(slot)))
	}
}

// resolveIssues 在寫入申請的同一個 transaction 內，把還沒處理的問題中這次寫入處理掉的標上時間：欄位用寫入路徑回報的 diff，
// 文件用寫入前後的證件（before、after）。
func resolveIssues(ctx context.Context, q *store.Queries, applicationID uuid.UUID, now time.Time, diff draftDiff, docsBefore, docsAfter []store.SellerKycDocument) error {
	open, err := q.UnresolvedKYCReviewIssues(ctx, applicationID)
	if err != nil || len(open) == 0 {
		return err
	}
	var resolved []uuid.UUID
	for _, issue := range open {
		if issueResolved(issue.Field, model.Deref(issue.DocumentAssetID), diff.fields, docsBefore, docsAfter) {
			resolved = append(resolved, issue.IssueID)
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return q.ResolveKYCReviewIssues(ctx, store.ResolveKYCReviewIssuesParams{IssueIds: resolved, Now: now})
}
