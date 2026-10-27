package main

import "github.com/Will413028/hawkr/backend/internal/modules/seller"

// 本機開發的種子店家（明文）：dev-seed 以執行中的 KYC 金鑰加密後寫入，種子檔不含密文。識別欄位用明顯是假的值。
//
// store_id 與 store_code 固定：infra/seed-data 的 04、05、06 寫死 store_id（Java 的資料庫），06 的 mer_id 是 HAWK 加 store_code，
// 01 的 account_roles 以 store_id 為 SELLER 的 source_id。account_id 與 store_id 刻意不同值（9、4 是帳號，store_id 取 1009、1004），
// 互換 ID 的缺陷才藏不住（同 01 的 buyer_id 規則）。logo 是固定、明顯是種子的 key 與 asset ID，不經 media。
const (
	devSeedStoreA, devSeedStoreB = 1009, 1004
)

// devSeedRebuiltAccounts 是 infra/seed-data/01-account.sql 刪除重建的帳號（9、4、買家 2001~2099、還沒開店的買家 3001~3099）：它們若擁有種子以外
// 的未關閉店家，重建帳號會讓店家失去 SELLER 角色，dev-seed 在前提檢查（-check）與寫入時都拒絕（已關閉的店家沒有角色，不擋）。dev_seed_test.go 檢查它與 01 的 SQL 一致。
var devSeedRebuiltAccounts = []seller.DevSeedAccountRange{{From: 4, To: 4}, {From: 9, To: 9}, {From: 2001, To: 2099}, {From: 3001, To: 3099}}

func devSeedStores() []seller.DevSeedStore {
	individual := seller.KYCIndividual
	return []seller.DevSeedStore{
		{
			StoreID: devSeedStoreA, StoreCode: 9, AccountID: 9,
			Name:         "翠綠有機農場",
			Description:  "嚴選台灣在地有機蔬果，產地直送，新鮮看得見。通過有機農產品驗證，堅持不使用農藥化肥。",
			ContactPhone: "0912345678", TownCode: "67000180", AddressDetail: "中山路88號",
			ContactName: "李翠綠", ContactMobile: "0912345678", SupportEmail: "seller1@hawkr.test",
			LogoKey: "public/seed/store/1009/logo.jpg", LogoAssetID: "seed-logo-1009",
			PayUniMerStatus: "1", PayUniMemberStatus: "1",
			FoodRegistration: "A-123456789-00001-1",
			KYC: seller.KYCDraftInput{
				SellerType: &individual, RealName: ptr("李翠綠"), BirthDate: ptr("1980-05-20"), IDCardNo: ptr("A123456789"),
				IDCardIssueDate: ptr("2020-03-15"), IDCardIssueType: ptr("換發"), IDCardIssueLocation: ptr("臺南市"),
				EnglishName: ptr("Green Organic Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr("123456789012"),
				ProductType: ptr("有機蔬果"),
			},
		},
		{
			StoreID: devSeedStoreB, StoreCode: 4, AccountID: 4,
			Name:         "澎湖鮮味水產",
			Description:  "澎湖直送野生海產，急速冷凍鎖住鮮味。每日漁船現撈，品質保證。",
			ContactPhone: "0923456789", TownCode: "10016010", AddressDetail: "中正路168號",
			ContactName: "陳海鮮", ContactMobile: "0923456789", SupportEmail: "seller2@hawkr.test",
			LogoKey: "public/seed/store/1004/logo.jpg", LogoAssetID: "seed-logo-1004",
			PayUniMerStatus: "1", PayUniMemberStatus: "1",
			PendingFoodRegistration: "A-987654321-00002-2",
			KYC: seller.KYCDraftInput{
				SellerType: &individual, RealName: ptr("陳海鮮"), BirthDate: ptr("1985-11-02"), IDCardNo: ptr("A123456789"),
				IDCardIssueDate: ptr("2021-06-01"), IDCardIssueType: ptr("換發"), IDCardIssueLocation: ptr("澎湖縣"),
				EnglishName: ptr("Penghu Fresh Seafood"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr("987654321098"),
				ProductType: ptr("水產海鮮"),
			},
		},
	}
}

func ptr[T any](v T) *T { return &v }
