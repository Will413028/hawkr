package model

import (
	"slices"
	"testing"
)

func completeStoreProfile() Profile {
	return Profile{
		TownCode: ptrTo("63000050"), AddressDetail: ptrTo("測試路一號"),
		ContactName: ptrTo("王小明"), ContactMobile: ptrTo("0912345678"), SupportEmail: ptrTo("a@example.com"), ContactPhone: ptrTo("0229010572"),
	}
}

func TestMissingStoreFieldsOnlyRequiresAddressAndSupportEmail(t *testing.T) {
	if got := MissingStoreFields(completeStoreProfile()); len(got) != 0 {
		t.Fatalf("齊全的店家缺 %v", got)
	}
	cases := map[string]struct {
		edit func(*Profile)
		want []string
	}{
		"沒有地址":       {func(p *Profile) { p.TownCode, p.AddressDetail = nil, nil }, []string{StoreFieldAddress}},
		"只有鄉鎮市區":     {func(p *Profile) { p.AddressDetail = nil }, []string{StoreFieldAddress}},
		"退役的鄉鎮市區代碼":  {func(p *Profile) { p.TownCode = ptrTo("99999999") }, []string{StoreFieldAddress}},
		"沒有客服信箱":     {func(p *Profile) { p.SupportEmail = nil }, []string{StoreFieldSupportEmail}},
		"聯絡人姓名是選填":   {func(p *Profile) { p.ContactName = nil }, []string{}},
		"聯絡人手機是選填":   {func(p *Profile) { p.ContactMobile = nil }, []string{}},
		"商店電話是選填":    {func(p *Profile) { p.ContactPhone = nil }, []string{}},
		"什麼都沒有依欄位順序": {func(p *Profile) { *p = Profile{} }, []string{StoreFieldAddress, StoreFieldSupportEmail}},
	}
	for name, tc := range cases {
		p := completeStoreProfile()
		tc.edit(&p)
		if got := MissingStoreFields(p); !slices.Equal(got, tc.want) {
			t.Errorf("%s：缺 %v，要 %v", name, got, tc.want)
		}
	}
}

func TestNewlyMissingOnlyCountsRequiredFieldsClearedNow(t *testing.T) {
	cleared := completeStoreProfile()
	cleared.SupportEmail, cleared.TownCode = nil, nil
	if got := NewlyMissing(completeStoreProfile(), cleared); !slices.Equal(got, []string{StoreFieldAddress, StoreFieldSupportEmail}) {
		t.Errorf("got %v", got)
	}
	if got := NewlyMissing(cleared, cleared); len(got) != 0 {
		t.Errorf("原本就缺的不算：%v", got)
	}
	if got := NewlyMissing(cleared, completeStoreProfile()); len(got) != 0 {
		t.Errorf("補齊不算：%v", got)
	}
	optional := completeStoreProfile()
	optional.ContactName, optional.ContactMobile = nil, nil
	if got := NewlyMissing(completeStoreProfile(), optional); len(got) != 0 {
		t.Errorf("清掉選填欄位不算：%v", got)
	}
}

func TestClearedLockedFieldsDependsOnApplicationStatus(t *testing.T) {
	clearedAll := completeStoreProfile()
	clearedAll.TownCode, clearedAll.AddressDetail, clearedAll.SupportEmail = nil, nil, nil
	clearedAll.ContactName, clearedAll.ContactMobile = nil, nil
	locked := []string{StoreFieldAddress, StoreFieldSupportEmail}
	cases := []struct {
		status KYCStatus
		want   []string
	}{
		{"", nil},
		{KYCStatusDraft, nil},
		{KYCStatusSubmitted, locked},
		{KYCStatusApproved, locked},
		{KYCStatusRejected, nil},
		{KYCStatusCancelled, nil},
	}
	for _, tc := range cases {
		got := ClearedLockedFields(completeStoreProfile(), clearedAll, tc.status)
		if !slices.Equal(got, tc.want) {
			t.Errorf("狀態 %q：不能清掉的是 %v，要 %v", tc.status, got, tc.want)
		}
	}
	optionalOnly := completeStoreProfile()
	optionalOnly.ContactName, optionalOnly.ContactMobile = nil, nil
	for _, status := range KYCStatusValues() {
		if got := ClearedLockedFields(completeStoreProfile(), optionalOnly, status); len(got) != 0 {
			t.Errorf("狀態 %q：聯絡人姓名與手機任何時候都能清掉：%v", status, got)
		}
	}
	if got := ClearedLockedFields(clearedAll, clearedAll, KYCStatusApproved); len(got) != 0 {
		t.Errorf("原本就缺的不算：%v", got)
	}
}
