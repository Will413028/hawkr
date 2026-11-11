package payuni

import (
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

func ptr(s string) *string { return &s }

func completeProfile() model.Profile {
	return model.Profile{
		TownCode: ptr("63000050"), AddressDetail: ptr("測試路一號"),
		ContactName: ptr("王小明"), ContactMobile: ptr("0912345678"), SupportEmail: ptr("a@example.com"), ContactPhone: ptr("0229010572"),
	}
}

func TestStoreDataMissingStillRequiresFourFields(t *testing.T) {
	if got := StoreDataOf(completeProfile()).Missing(); len(got) != 0 {
		t.Fatalf("齊全的店家缺 %v", got)
	}
	cases := map[string]struct {
		edit func(*model.Profile)
		want []string
	}{
		"沒有地址":      {func(p *model.Profile) { p.TownCode, p.AddressDetail = nil, nil }, []string{model.StoreFieldAddress}},
		"只有鄉鎮市區":    {func(p *model.Profile) { p.AddressDetail = nil }, []string{model.StoreFieldAddress}},
		"退役的鄉鎮市區代碼": {func(p *model.Profile) { p.TownCode = ptr("99999999") }, []string{model.StoreFieldAddress}},
		"沒有聯絡人":     {func(p *model.Profile) { p.ContactName = nil }, []string{model.StoreFieldContactName}},
		"沒有手機與信箱":   {func(p *model.Profile) { p.ContactMobile, p.SupportEmail = nil, nil }, []string{model.StoreFieldContactMobile, model.StoreFieldSupportEmail}},
		"商店電話不是必填":  {func(p *model.Profile) { p.ContactPhone = nil }, []string{}},
		"全部都沒有":     {func(p *model.Profile) { *p = model.Profile{} }, []string{model.StoreFieldAddress, model.StoreFieldContactName, model.StoreFieldContactMobile, model.StoreFieldSupportEmail}},
	}
	for name, tc := range cases {
		p := completeProfile()
		tc.edit(&p)
		if got := StoreDataOf(p).Missing(); !slices.Equal(got, tc.want) {
			t.Errorf("%s：缺 %v，要 %v", name, got, tc.want)
		}
	}
}

// 同步到 PayUni 的店家欄位只有 StoreData 一份定義：每個欄位改變都被偵測到，不相干的欄位不算。
func TestSyncedFieldsChangedCoversEveryPayUniField(t *testing.T) {
	for name, edit := range map[string]func(*model.Profile){
		"鄉鎮市區": func(p *model.Profile) { p.TownCode = ptr("64000120") }, "地址明細": func(p *model.Profile) { p.AddressDetail = ptr("別條路") },
		"聯絡人": func(p *model.Profile) { p.ContactName = ptr("李大華") }, "聯絡手機": func(p *model.Profile) { p.ContactMobile = ptr("0987654321") },
		"客服信箱": func(p *model.Profile) { p.SupportEmail = ptr("b@example.com") }, "商店電話": func(p *model.Profile) { p.ContactPhone = nil },
	} {
		after := completeProfile()
		edit(&after)
		if !SyncedFieldsChanged(completeProfile(), after) {
			t.Errorf("%s 改變沒有被偵測到", name)
		}
	}
	after := completeProfile()
	after.StoreName, after.Description, after.Prize = "別的店名", ptr("別的介紹"), ptr("金牌")
	if SyncedFieldsChanged(completeProfile(), after) {
		t.Error("店名、介紹與得獎紀錄不是同步欄位")
	}
}
