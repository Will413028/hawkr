package model

import (
	"slices"

	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// 店家上可以指出「有問題」的欄位路徑（與對外 Store 的欄位一對一）：送出 KYC 之前還缺的店家欄位與金流商問題要補的店家欄位
// 都用這一套。
const (
	StoreFieldAddress       = "address"
	StoreFieldContactName   = "contact_name"
	StoreFieldContactMobile = "contact_mobile"
	StoreFieldSupportEmail  = "support_email"
	StoreFieldContactPhone  = "contact_phone"
)

// StoreFieldNames 回傳全部店家欄位路徑，依對外 StoreField 的數字順序。
func StoreFieldNames() []string {
	return []string{StoreFieldAddress, StoreFieldContactName, StoreFieldContactMobile, StoreFieldSupportEmail, StoreFieldContactPhone}
}

// MissingStoreFields 回傳送出 KYC 之前店家還缺（或不合法）的必填欄位路徑，依 StoreFieldNames 的順序：地址（現行的鄉鎮市區加詳細地址）
// 與客服信箱。聯絡人姓名、聯絡人手機與商店電話是選填。物流要用地址與客服信箱聯絡店家，送出前的檢查與「不能清掉」的檢查都用它，必填的規則只有這一份。
func MissingStoreFields(p Profile) []string {
	out := []string{}
	if _, ok := twaddr.TownByCode(Deref(p.TownCode)); !ok || Deref(p.AddressDetail) == "" {
		out = append(out, StoreFieldAddress)
	}
	if Deref(p.SupportEmail) == "" {
		out = append(out, StoreFieldSupportEmail)
	}
	return out
}

// NewlyMissing 回傳 after 比 before 多出來的缺項：原本有、被清掉（或改成不合法）的必填欄位。
func NewlyMissing(before, after Profile) []string {
	was := MissingStoreFields(before)
	out := []string{}
	for _, f := range MissingStoreFields(after) {
		if !slices.Contains(was, f) {
			out = append(out, f)
		}
	}
	return out
}

// ClearedLockedFields 回傳 after 相對 before 新缺、而且在申請狀態 status 下不能清掉的必填店家欄位：申請審核中或已核准時，
// 審核與物流都依地址與客服信箱聯絡店家，不能清掉；其他狀態（含還沒有申請，status 為空）可以清，送出前會再檢查完整性。
// 聯絡人姓名與手機任何時候都能清掉。
func ClearedLockedFields(before, after Profile, status KYCStatus) []string {
	if status != KYCStatusSubmitted && status != KYCStatusApproved {
		return nil
	}
	if cleared := NewlyMissing(before, after); len(cleared) > 0 {
		return cleared
	}
	return nil
}
