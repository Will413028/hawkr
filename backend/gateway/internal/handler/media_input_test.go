package handler

import (
	"testing"
)

// TestResolveInputObjectKey 覆蓋輸入正規化小函式（供後續切片其他輸入面複用）。
func TestResolveInputObjectKey(t *testing.T) {
	const mediaBase = "https://test-media.example.com"
	tests := []struct {
		name    string
		keyVal  string
		urlVal  string
		want    string
		wantErr bool
	}{
		{"裸 key（只給 key）", "public/banner/1/20260717/x.jpg", "", "public/banner/1/20260717/x.jpg", false},
		{"裸 key（只給 url，合法）", "", "public/banner/1/20260717/x.jpg", "public/banner/1/20260717/x.jpg", false},
		{"媒體域名 URL 剝殼（url 欄，模擬 GET 回應回打）", "", "https://test-media.example.com/public/banner/1/20260717/x.jpg", "public/banner/1/20260717/x.jpg", false},
		{"MediaBaseURL 剝殼（key 欄也走同規則）", "https://test-media.example.com/public/banner/1/x.jpg", "", "public/banner/1/x.jpg", false},
		{"外來域名 400", "", "https://evil.com/public/x.jpg", "", true},
		{"路徑穿越 .. 400", "", "public/../secret/x.jpg", "", true},
		{"前綴不符白名單 400（tmp/）", "", "tmp/staging/x.jpg", "", true},
		{"key 與 url 皆給且一致 → 通過", "public/banner/1/x.jpg", "https://test-media.example.com/public/banner/1/x.jpg", "public/banner/1/x.jpg", false},
		{"key 與 url 不一致 → 400", "public/banner/1/a.jpg", "public/banner/1/b.jpg", "", true},
		{"皆空 → 空字串（交由 staged token / 必填處理）", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveInputObjectKey(mediaBase, tt.keyVal, tt.urlVal, "public/")
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveInputObjectKey(%q,%q) err=%v wantErr=%v", tt.keyVal, tt.urlVal, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("resolveInputObjectKey(%q,%q) = %q, want %q", tt.keyVal, tt.urlVal, got, tt.want)
			}
		})
	}
}
