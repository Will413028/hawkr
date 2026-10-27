package grpc

import (
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

func sp(s string) *string { return &s }

// #3031：三段地址分開回。
//
// 🔴 這裡的風險是**舊資料**：address_city 落地之前，business_address 是串接後的整段，
// 那是它當初唯一存得下來的形態。一律拆的話，2026-09-02 之前送過稅籍的賣家會在 S02
// 看到一個少了縣市與行政區的地址——而那是他當初填對的資料，畫面上沒有任何錯誤。
func TestAddressDetailOf(t *testing.T) {
	cases := []struct {
		name string
		p    domain.SellerTaxProfile
		want string
	}{
		{
			name: "新資料：整段減去前兩段",
			p: domain.SellerTaxProfile{
				BusinessAddress: sp("台北市大安區忠孝東路一段1號"),
				AddressCity:     sp("台北市"),
				AddressDistrict: sp("大安區"),
			},
			want: "忠孝東路一段1號",
		},
		{
			// 🔴 陰性對照：沒有 city 就整段原樣回。
			name: "舊資料（address_city 為 NULL）：整段原樣回",
			p: domain.SellerTaxProfile{
				BusinessAddress: sp("台北市大安區忠孝東路一段1號"),
			},
			want: "台北市大安區忠孝東路一段1號",
		},
		{
			name: "只有 city 沒有 district：只減去 city",
			p: domain.SellerTaxProfile{
				BusinessAddress: sp("台北市忠孝東路一段1號"),
				AddressCity:     sp("台北市"),
			},
			want: "忠孝東路一段1號",
		},
		{
			name: "全空",
			p:    domain.SellerTaxProfile{},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := addressDetailOf(tc.p); got != tc.want {
				t.Fatalf("addressDetailOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// toSellerTaxIdentity 三格各歸各位，且新舊資料都不掉東西。
func TestToSellerTaxIdentity_SplitsAddress(t *testing.T) {
	got := toSellerTaxIdentity(domain.SellerTaxProfile{
		BusinessAddress: sp("台北市大安區忠孝東路一段1號"),
		AddressCity:     sp("台北市"),
		AddressDistrict: sp("大安區"),
	})
	if got.GetAddressCity() != "台北市" || got.GetAddressDistrict() != "大安區" {
		t.Fatalf("🔴 前兩格沒填：city=%q district=%q —— 賣家重新編輯時它們會是空的",
			got.GetAddressCity(), got.GetAddressDistrict())
	}
	if got.GetAddressDetail() != "忠孝東路一段1號" {
		t.Fatalf("detail 應只剩詳細地址，實得 %q", got.GetAddressDetail())
	}

	// 舊資料：三格加起來仍等於原本那一整段，一個字都不能少。
	old := toSellerTaxIdentity(domain.SellerTaxProfile{
		BusinessAddress: sp("台北市大安區忠孝東路一段1號"),
	})
	if old.GetAddressCity()+old.GetAddressDistrict()+old.GetAddressDetail() != "台北市大安區忠孝東路一段1號" {
		t.Fatalf("舊資料串回來對不上：%q+%q+%q",
			old.GetAddressCity(), old.GetAddressDistrict(), old.GetAddressDetail())
	}
}
