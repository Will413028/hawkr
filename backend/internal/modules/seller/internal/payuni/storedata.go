package payuni

import (
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// StoreData 是店家上送給 PayUni 的資料：地址、聯絡人姓名與手機、客服信箱與商店電話。「哪些店家欄位會同步到 PayUni」只在這個型別定義：
// 變更偵測（SyncedFieldsChanged）、完整性（Missing）、bind 與 update 的請求都由它推導，新增一個欄位只改這裡。沒有值的欄位是空字串。
type StoreData struct {
	TownCode, AddressDetail                  string
	ContactName, ContactMobile, SupportEmail string
	ContactPhone                             string
}

// StoreDataOf 取出店家 profile 上會同步到 PayUni 的資料。
func StoreDataOf(p model.Profile) StoreData {
	return StoreData{
		TownCode: model.Deref(p.TownCode), AddressDetail: model.Deref(p.AddressDetail),
		ContactName: model.Deref(p.ContactName), ContactMobile: model.Deref(p.ContactMobile), SupportEmail: model.Deref(p.SupportEmail),
		ContactPhone: model.Deref(p.ContactPhone),
	}
}

// SyncedFieldsChanged 回答兩份 profile 之間有沒有會同步到 PayUni 的資料改變。
func SyncedFieldsChanged(before, after model.Profile) bool {
	return StoreDataOf(before) != StoreDataOf(after)
}

// Address 把地址組成 PayUni 要的文字（縣市加鄉鎮市區加詳細地址）與 3 碼郵遞區號；沒有地址、沒有明細或代碼不是現行的鄉鎮市區時 ok 為 false。
func (d StoreData) Address() (text, postal string, ok bool) {
	town, found := twaddr.TownByCode(d.TownCode)
	if !found || d.AddressDetail == "" {
		return "", "", false
	}
	return town.CountyName + town.Name + d.AddressDetail, town.PostalCode, true
}

// Missing 回傳 PayUni 綁定需要、而店家還沒有（或不合法）的欄位路徑，依 model.StoreFieldNames 的順序：地址、聯絡人姓名、聯絡人手機、
// 客服信箱（商店電話不是必填）。送出 KYC 的檢查、綁定與同步時的完整性檢查、清除必填欄位的檢查都用它，完整性的規則只有這一份。
func (d StoreData) Missing() []string {
	out := []string{}
	if _, _, ok := d.Address(); !ok {
		out = append(out, model.StoreFieldAddress)
	}
	for _, f := range []struct{ value, field string }{
		{d.ContactName, model.StoreFieldContactName}, {d.ContactMobile, model.StoreFieldContactMobile}, {d.SupportEmail, model.StoreFieldSupportEmail},
	} {
		if f.value == "" {
			out = append(out, f.field)
		}
	}
	return out
}

// MissingStoreFields 是 StoreDataOf(p).Missing()。
func MissingStoreFields(p model.Profile) []string { return StoreDataOf(p).Missing() }

// NewlyMissing 回傳 after 比 before 多出來的缺項：原本有、被清掉（或改成不合法）的必填欄位。
func NewlyMissing(before, after model.Profile) []string {
	was := MissingStoreFields(before)
	out := []string{}
	for _, f := range MissingStoreFields(after) {
		if !contains(was, f) {
			out = append(out, f)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
