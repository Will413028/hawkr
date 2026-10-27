package domain

import (
	"net/url"
	"strings"
)

// ProspectContactInput 是一筆站外業者的原始聯絡欄位，直接取自 BD 名單或 admin 表單。
type ProspectContactInput struct {
	TaxID   string
	Email   string
	Phone   string
	Website string
	Social  string
}

// ProspectMatchKeys 是正規化後的識別比對鍵組。
// 空字串代表「這個維度沒有可用的鍵」——呼叫端看到空字串必須跳過，
// 絕不可以拿空字串進 WHERE。這是本型別唯一的不變式。
type ProspectMatchKeys struct {
	TaxID  string
	Email  string
	Phone  string
	Domain string
	Social string
}

// prospectPlaceholders 是 BD 名單裡代表「還沒查到」的佔位值（小寫比對）。
// 匯入來源實測：email 541 筆、電話 48 筆填「待確認」。不排除它們，
// 這幾百筆會共用同一個鍵、在 T2 比對下塌成同一家業者。
var prospectPlaceholders = map[string]struct{}{
	"":    {},
	"待確認": {},
	"n/a": {},
	"na":  {},
	"-":   {},
	"無":   {},
	"未知":  {},
}

// prospectSocialHosts 把社群平台網域對應到平台代號。落在這張表裡的 host
// 絕不可以成為 match_domain——匯入來源 CRM1 的 454 筆連結全部落在這裡
// （threads.com 347、facebook.com 107），用網域當鍵會把 347 家併成一家。
// 值為空字串代表「這個 host 連 handle 都不具識別力」（短網址），整筆丟棄。
var prospectSocialHosts = map[string]string{
	"threads.com":    "threads",
	"threads.net":    "threads",
	"facebook.com":   "facebook",
	"m.facebook.com": "facebook",
	"fb.com":         "facebook",
	"instagram.com":  "instagram",
	"line.me":        "line",
	"shopee.tw":      "shopee",
	"lihi.cc":        "",
}

// prospectNonProfilePaths 是平台路由名，絕不是帳號／業者身分。
// URL 路徑恰好是這些段落之一時（/p/xxx、/t/yyy、/pages/name/id 的 /pages），
// 代表指向的是貼文、頁面、collection，而非單一帳號。保守策略：丟棄。
var prospectNonProfilePaths = map[string]struct{}{
	"p":           {}, // instagram posts
	"reel":        {}, // instagram reels
	"t":           {}, // threads posts
	"pages":       {}, // facebook pages
	"groups":      {}, // facebook groups
	"watch":       {}, // facebook watch
	"explore":     {}, // instagram explore
	"share":       {}, // share pages
	"stories":     {}, // stories pages
	"profile.php": {}, // facebook old-style (will be handled specially)
}

// normalizeToken 是所有欄位共用的第一道：去空白、轉小寫、佔位值歸零。
func normalizeToken(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if _, isPlaceholder := prospectPlaceholders[v]; isPlaceholder {
		return ""
	}
	return v
}

// normalizePhone 只保留數字，讓 07-610-7551 與 07-6107551 得到同一個鍵。
// 開頭的台灣國碼 886 換回 0，否則 +886-2-… 與 02-… 會被當成兩家。
func normalizePhone(s string) string {
	v := normalizeToken(s)
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if strings.HasPrefix(digits, "886") {
		digits = "0" + strings.TrimPrefix(digits, "886")
	}
	return digits
}

// parseProspectURL 把一個網址解析成「可註冊網域」或「社群 handle」二選一。
// 兩者互斥：社群網域一律只出 social，永遠不出 domain。
//
// 保守策略：遇到不確定的身分，寧可回傳空鍵而非錯誤共用鍵。
// 空鍵代表「沒有這個維度可用」，不會造成 collapse；
// 錯的共用鍵會把數百家無關業者塌成同一家，這是災難級故障。
func parseProspectURL(raw string) (domain, social string) {
	v := normalizeToken(raw)
	if v == "" {
		return "", ""
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", ""
	}
	host := strings.TrimPrefix(u.Host, "www.")
	platform, isSocial := prospectSocialHosts[host]
	if !isSocial {
		return host, ""
	}
	if platform == "" {
		return "", "" // 短網址：host 與 handle 都不具識別力
	}
	handle := strings.Trim(u.Path, "/")
	if handle == "" {
		return "", "" // 裸網域，不是帳號身分
	}
	// 路徑必須恰好是一段。多段路徑（/pages/Name/ID、/p/PostID 等）絕不是帳號身分。
	if strings.Contains(handle, "/") {
		return "", "" // 深層路徑，不是單一帳號
	}
	handle = strings.TrimPrefix(handle, "@")
	if handle == "" {
		return "", ""
	}
	// Facebook profile.php 特例：身分在 query 參數裡。其他平台 profile.php 直接丟棄。
	if platform == "facebook" && strings.EqualFold(handle, "profile.php") {
		idParam := u.Query().Get("id")
		if idParam == "" {
			return "", ""
		}
		return "", "facebook:" + idParam
	}
	// 單一路徑段卻是平台路由名（p、t、pages 等），絕不是帳號身分。
	if _, isRoute := prospectNonProfilePaths[strings.ToLower(handle)]; isRoute {
		return "", ""
	}
	return "", platform + ":" + handle
}

// BuildProspectMatchKeys 是本套件唯一的進入點：原始欄位進、正規化鍵組出。
// 遷移與 admin 建檔共用它（spec D5：一個解析器兩個呼叫端）。
func BuildProspectMatchKeys(in ProspectContactInput) ProspectMatchKeys {
	keys := ProspectMatchKeys{
		TaxID: normalizeToken(in.TaxID),
		Email: normalizeToken(in.Email),
		Phone: normalizePhone(in.Phone),
	}
	keys.Domain, keys.Social = parseProspectURL(in.Website)
	if keys.Social == "" {
		if _, s := parseProspectURL(in.Social); s != "" {
			keys.Social = s
		}
	}
	return keys
}
