package moa

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SplitCropName 把官方作物名拆成大類與子品項。
// MOA 的命名慣例是「大類-子品項」，無連字號時整串即為大類。
//
// 用第一個連字號（strings.Index）而非最後一個：子品項本身可能帶連字號
// （例如「甘薯-臺農57號-紅心」的子品項是「臺農57號-紅心」，大類仍是
// 「甘薯」），用最後一個連字號切會把子品項的一部分誤併進大類名。
//
// idx == 0（連字號在開頭，例如「-初秋」）視同沒有有效的「大類-子品項」
// 結構，整串當大類——連字號前沒有任何字元不算真的切出了大類，寫一個
// 空字串當大類名會產生一個沒有名字的作物分類，比「沒切開」更糟。
func SplitCropName(name string) (group, variant string) {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "-")
	if idx <= 0 {
		return name, ""
	}
	return strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+1:])
}

// GroupCode 由大類名與分類代碼產生穩定的大類識別碼。
// 用 hash 而非流水號，確保重跑 ingest 得到同一個 code。
func GroupCode(groupName, categoryCode string) string {
	sum := sha256.Sum256([]byte(groupName + "\x00" + categoryCode))
	return "moa-group:" + hex.EncodeToString(sum[:8])
}
