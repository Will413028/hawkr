// Package twaddr 是台灣的縣市與鄉鎮市區代碼：內政部戶政的鄉鎮市區代碼（8 碼 towncode，前 5 碼是縣市代碼），
// 以及每個鄉鎮市區的 3 碼郵遞區號。
//
// 來源：國土測繪中心代碼服務（https://api.nlsc.gov.tw/other/ListCounty、/other/ListTown1/{A..Z}），2026-10-01 下載，
// 22 個縣市、368 個鄉鎮市區，原樣存在 moi-towns.csv。郵遞區號 postal-codes.csv 是以縣市加鄉鎮市區的名稱（台／臺
// 視為相同）對照中華郵政 3 碼郵遞區號表得出。towns_gen.go 由 generate.go 從這兩份 CSV 產生，不得手改；行政區劃調整
// 時更新 CSV 後重新產生。
package twaddr

import (
	"strings"
	"unicode/utf8"
)

//go:generate go run generate.go

// Town 是一個鄉鎮市區。名稱照內政部的寫法（臺）。
type Town struct {
	// Code 是 8 碼的鄉鎮市區代碼。
	Code string
	Name string
	// CountyCode 是 5 碼的縣市代碼，等於 Code 的前 5 碼。
	CountyCode string
	CountyName string
	// PostalCode 是 3 碼郵遞區號。
	PostalCode string
}

// County 是一個縣市。
type County struct {
	Code string
	Name string
}

var byCode = func() map[string]Town {
	m := make(map[string]Town, len(towns))
	for _, t := range towns {
		m[t.Code] = t
	}
	return m
}()

// TownByCode 以 8 碼代碼找鄉鎮市區；不是現行的代碼時 ok 為 false。
func TownByCode(code string) (Town, bool) {
	t, ok := byCode[code]
	return t, ok
}

// Towns 回傳全部鄉鎮市區，依郵遞區號（同號依代碼）。
func Towns() []Town { return append([]Town(nil), towns...) }

// Counties 回傳全部縣市，依其鄉鎮市區最小的郵遞區號（即在 Towns 中第一次出現的順序）。
func Counties() []County {
	var out []County
	seen := map[string]bool{}
	for _, t := range towns {
		if !seen[t.CountyCode] {
			seen[t.CountyCode] = true
			out = append(out, County{Code: t.CountyCode, Name: t.CountyName})
		}
	}
	return out
}

// Split 在地址開頭是現行的「縣市＋鄉鎮市區」（台與臺視為相同）時拆出鄉鎮市區代碼與其後的地址（去掉前後空白，其後的
// 文字照原樣保留）；不是（例如科學園區的地址）或有一個以上的鄉鎮市區符合時 ok 為 false：不猜，由呼叫方決定怎麼處理。
func Split(address string) (townCode, detail string, ok bool) {
	trimmed := strings.TrimSpace(address)
	normalized := strings.NewReplacer("台", "臺").Replace(trimmed)
	var match Town
	matches := 0
	for _, t := range towns {
		if strings.HasPrefix(normalized, t.CountyName+t.Name) {
			match, matches = t, matches+1
		}
	}
	if matches != 1 {
		return "", "", false
	}
	// 台與臺一字換一字，字數不變，地址其後的文字照原樣保留。
	return match.Code, strings.TrimSpace(string([]rune(trimmed)[utf8.RuneCountInString(match.CountyName+match.Name):])), true
}
