package media

import (
	"errors"
	"fmt"
	"strings"
)

// refTypes 是 media-service 接受的 ref_type 封閉集合：ref_type.go（由 CodeMeta 生成）的全部常數。手寫在這裡是因為生成檔只輸出
// 常數；測試（TestRefTypesMatchTheGeneratedConstants）解析 ref_type.go 比對，常數增減時這裡漏改會失敗。
var refTypes = map[string]bool{
	RefTypeProduct: true, RefTypeProductDesc: true, RefTypeSellerLogo: true, RefTypeShipping: true, RefTypeSellerCover: true,
	RefTypeIdentityVerification: true, RefTypeChatMessage: true, RefTypePost: true, RefTypeBanner: true, RefTypeAftersale: true,
	RefTypeCertification: true, RefTypeBuyerAvatar: true, RefTypeOrderReceipt: true, RefTypeAiStyle: true, RefTypeOrderReview: true,
	RefTypeProductImportPreview: true, RefTypeProductImportPreviewDesc: true, RefTypeGovernanceEvidence: true, RefTypeRecipe: true,
	RefTypeInvoiceDocument: true, RefTypeDeliveryPod: true, RefTypeSellerProspectLogo: true, RefTypeSellerProspectCover: true,
	RefTypeOther: true,
}

// CheckFinalizeArgs 檢查 Media.Finalize 的參數是呼叫方該給的：上傳者存在、ref_type 屬於封閉集合、token 非空白。回錯誤表示
// 呼叫方的程式錯誤（不是使用者的 token 問題），Media 的實作（media-service 轉接與 Fake）在送出之前都用它擋下，所以實作回的
// ErrStagedTokenRejected 只可能是 token 本身被拒絕。
func CheckFinalizeArgs(u Uploader, stagedToken, refType string) error {
	switch {
	case u.ID <= 0:
		return errors.New("缺少上傳者")
	case !refTypes[refType]:
		return fmt.Errorf("無效的 ref_type %q", refType)
	case strings.TrimSpace(stagedToken) == "":
		return errors.New("staged token 是空白")
	}
	return nil
}
