package model

import (
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/phone"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// Text 把輸入轉成儲存的值：去掉前後空白，空字串是 nil（沒有值）。
func Text(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// Required 去掉前後空白後必須是 1 到 limit 字；field 是模組輸入的欄位名，name 是錯誤訊息裡的名稱。
func Required(value string, limit int, field, name string) (string, error) {
	value = strings.TrimSpace(value)
	if n := utf8.RuneCountInString(value); n == 0 || n > limit {
		return "", apperr.InvalidInputOn([]string{field}, "%s必須是 1 到 %d 字", name, limit)
	}
	return value, nil
}

// Optional 去掉前後空白，空字串是 nil；超過 limit 字回錯誤。
func Optional(value string, limit int, field, name string) (*string, error) {
	text := Text(value)
	if text != nil && utf8.RuneCountInString(*text) > limit {
		return nil, apperr.InvalidInputOn([]string{field}, "%s不可超過 %d 字", name, limit)
	}
	return text, nil
}

// OneOf 要求 value 是 values 的其中一個。
func OneOf[T ~string](value T, values []T, field, message string) error {
	if !slices.Contains(values, value) {
		return apperr.InvalidInputOn([]string{field}, "%s", message)
	}
	return nil
}

// NoteBodyMaxRunes 是內部註記內容的字數上限。
const NoteBodyMaxRunes = 2000

// NoteBody 驗證內部註記的內容：去掉前後空白後 1 到 NoteBodyMaxRunes 字（措施的註記與案件的註記共用；field 是模組輸入的欄位名）。
func NoteBody(body, field string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if n := utf8.RuneCountInString(trimmed); n == 0 || n > NoteBodyMaxRunes {
		return "", apperr.InvalidInputOn([]string{field}, "%s", text.NoteBodyLength().Text)
	}
	return trimmed, nil
}

// Deref 是 *string 的值，nil 是空字串。
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FirstNonEmpty 回傳第一個去掉空白後不是空字串的值（回傳原值，不去空白）；全部是空的回傳空字串。
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Normalize 驗證並正規化合併後的 profile：一次檢查全部欄位，任何一欄不合法就不寫入。與 DB 的 CHECK
// 是同一組規則，DB 是保底。
func Normalize(p *Profile) error {
	p.StoreName = trimtext.Trim(p.StoreName)
	if n := utf8.RuneCountInString(p.StoreName); n == 0 || n > 100 {
		return apperr.InvalidInputOn([]string{"StoreName"}, "店名必須是 1 到 100 字")
	}
	if err := maxRunes(p.Description, 2000, "Description", "店家介紹不可超過 2000 字"); err != nil {
		return err
	}
	if p.ContactPhone != nil {
		normalized := phone.NormalizeToLocal(*p.ContactPhone)
		if !phone.Valid(normalized) {
			return apperr.InvalidInputOn([]string{"ContactPhone"}, "聯絡電話不合法")
		}
		p.ContactPhone = &normalized
	}
	if err := normalizeContact(p); err != nil {
		return err
	}
	if err := normalizeAddress(p); err != nil {
		return err
	}
	if err := maxRunes(p.Prize, 255, "Prize", "得獎紀錄不可超過 255 字"); err != nil {
		return err
	}
	if p.WebsiteURL != nil {
		normalized, ok := websiteURL(*p.WebsiteURL)
		if !ok {
			return apperr.InvalidInputOn([]string{"WebsiteURL"}, "網站網址格式不正確")
		}
		p.WebsiteURL = &normalized
	}
	return nil
}

// 店家聯絡欄位的長度上限就是 PayUni 的上限（代理商文件 §7.3：ContactName 20 字、MerEmail 50 字）：PayUni 是這些欄位唯一的去處，
// 再接第二家金流商時重新評估（見 CARRYOVER）。
const (
	ContactNameMaxRunes  = 20
	SupportEmailMaxRunes = 50
)

// normalizeContact 驗證並正規化送給 PayUni 的聯絡資料：聯絡人姓名至多 20 字、聯絡人手機是 09 開頭的 10 碼、客服信箱格式正確。
func normalizeContact(p *Profile) error {
	if p.ContactName != nil {
		name := trimtext.Trim(*p.ContactName)
		if n := utf8.RuneCountInString(name); n == 0 || n > ContactNameMaxRunes {
			return apperr.InvalidInputOn([]string{"ContactName"}, "聯絡人姓名必須是 1 到 %d 字", ContactNameMaxRunes)
		}
		p.ContactName = &name
	}
	if p.ContactMobile != nil {
		mobile, ok := NormalizeMobile(*p.ContactMobile)
		if !ok {
			return apperr.InvalidInputOn([]string{"ContactMobile"}, "聯絡人手機必須是 09 開頭的 10 碼手機")
		}
		p.ContactMobile = &mobile
	}
	if p.SupportEmail != nil {
		email, ok := NormalizeEmail(*p.SupportEmail)
		if !ok || utf8.RuneCountInString(email) > SupportEmailMaxRunes {
			return apperr.InvalidInputOn([]string{"SupportEmail"}, "客服電子信箱格式不正確，至多 %d 字", SupportEmailMaxRunes)
		}
		p.SupportEmail = &email
	}
	return nil
}

// normalizeAddress 驗證鄉鎮市區代碼與地址：代碼必須是現行的 8 碼鄉鎮市區代碼，地址要有鄉鎮市區。
func normalizeAddress(p *Profile) error {
	if p.TownCode != nil {
		if _, ok := twaddr.TownByCode(*p.TownCode); !ok {
			return apperr.InvalidInputOn([]string{"TownCode"}, "鄉鎮市區代碼不合法")
		}
	}
	if p.AddressDetail != nil {
		if p.TownCode == nil {
			return apperr.InvalidInputOn([]string{"AddressDetail"}, "填寫地址前要先選鄉鎮市區")
		}
		if utf8.RuneCountInString(*p.AddressDetail) > 255 {
			return apperr.InvalidInputOn([]string{"AddressDetail"}, "地址不可超過 255 字")
		}
	}
	return nil
}

// websiteURL 只接受有主機的 http、https 網址，scheme 與主機轉成小寫（DB 的 stores_website_url_check 區分大小寫）。
func websiteURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", false
	}
	normalized := u.String()
	return normalized, len(normalized) <= 500
}

func maxRunes(value *string, limit int, field, message string) error {
	if value != nil && utf8.RuneCountInString(*value) > limit {
		return apperr.InvalidInputOn([]string{field}, "%s", message)
	}
	return nil
}

// LikePattern 把使用者輸入的關鍵字轉成 ILIKE 的子字串樣式（SQL 要 ESCAPE '\'）：跳脫 \、% 與 _，兩端加 %；空白的關鍵字
// 是 nil（不篩選）。
func LikePattern(keyword string) *string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(keyword) + "%"
	return &pattern
}
