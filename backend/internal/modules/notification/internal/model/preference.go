package model

// Enabled 回報受眾 a 的收件人此刻是否收到分類 c 在渠道 ch 的通知：不可關的一律收到，其餘依 overrides（該收件人在這個分類下各渠道的覆寫列），
// 沒有覆寫列就是預設。意圖管線與公告派送共用這一個判斷。
func Enabled(a Audience, c PrefCategory, ch Channel, overrides map[Channel]bool) bool {
	if !Disableable(a, c, ch) {
		return true
	}
	if v, ok := overrides[ch]; ok {
		return v
	}
	return DefaultEnabled(c, ch)
}
