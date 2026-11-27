package main

import (
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

const (
	devSeedStoreA, devSeedStoreB = 1009, 1004
)

var devSeedAccountIDs = []int64{4, 9, 2001, 2002, 3001}

func devSeedStores() []seller.DevSeedStore {
	individual := seller.KYCIndividual
	return []seller.DevSeedStore{
		{
			StoreID: devSeedStoreA, StoreCode: 9, AccountID: 9,
			Name:         "翠綠有機農場",
			Description:  "嚴選台灣在地有機蔬果，產地直送，新鮮看得見。通過有機農產品驗證，堅持不使用農藥化肥。",
			ContactPhone: "0912345678", TownCode: "67000180", AddressDetail: "中山路88號",
			ContactName: "李翠綠", ContactMobile: "0912345678", SupportEmail: "seller1@hawkr.test",
			LogoKey: "public/seed/store/1009/logo.jpg", LogoAssetID: uuid.MustParse("0192a000-0000-7000-8000-000000001009"),
			KYC: seller.KYCDraftInput{
				SellerType: &individual, ProductType: ptr("有機蔬果"),
			},
		},
		{
			StoreID: devSeedStoreB, StoreCode: 4, AccountID: 4,
			Name:         "澎湖鮮味水產",
			Description:  "澎湖直送野生海產，急速冷凍鎖住鮮味。每日漁船現撈，品質保證。",
			ContactPhone: "0923456789", TownCode: "10016010", AddressDetail: "中正路168號",
			ContactName: "陳海鮮", ContactMobile: "0923456789", SupportEmail: "seller2@hawkr.test",
			LogoKey: "public/seed/store/1004/logo.jpg", LogoAssetID: uuid.MustParse("0192a000-0000-7000-8000-000000001004"),
			KYC: seller.KYCDraftInput{
				SellerType: &individual, ProductType: ptr("水產海鮮"),
			},
		},
	}
}

func ptr[T any](v T) *T { return &v }
