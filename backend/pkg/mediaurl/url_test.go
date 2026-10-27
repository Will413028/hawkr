package mediaurl

import "testing"

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		objectKey string
		want      string
	}{
		// Image Gateway 契約：base + "/i/original/" + objectKey（49 §6.1/§6.5）
		{"Image Gateway 原圖路徑", "https://media.example.com", "public/product/20260304/x.jpg", "https://media.example.com/i/original/public/product/20260304/x.jpg"},
		{"空 objectKey", "https://cdn.example.com", "", ""},
		{"http 前綴直接回傳", "https://cdn.example.com", "https://lh3.google.com/avatar.jpg", "https://lh3.google.com/avatar.jpg"},
		{"OSS key 走 original 路徑", "https://cdn.example.com", "uploads/avatar/123.jpg", "https://cdn.example.com/i/original/uploads/avatar/123.jpg"},
		{"base URL 尾部有斜線", "https://cdn.example.com/", "uploads/avatar/123.jpg", "https://cdn.example.com/i/original/uploads/avatar/123.jpg"},
		{"http 前綴（非 https）", "https://cdn.example.com", "http://other.com/img.jpg", "http://other.com/img.jpg"},
		{"base 未設時回裸 key（social/review 未配置域名的既有行為）", "", "uploads/avatar/123.jpg", "uploads/avatar/123.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildURL(tt.baseURL, tt.objectKey)
			if got != tt.want {
				t.Errorf("BuildURL(%q, %q) = %q, want %q", tt.baseURL, tt.objectKey, got, tt.want)
			}
		})
	}
}

func TestBuildImageURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		preset    string
		objectKey string
		want      string
	}{
		{"preset 拼接", "https://media.example.com", "thumb", "public/product/x.jpg", "https://media.example.com/i/thumb/public/product/x.jpg"},
		{"original preset", "https://media.example.com", "original", "a/b.jpg", "https://media.example.com/i/original/a/b.jpg"},
		{"空 objectKey", "https://media.example.com", "thumb", "", ""},
		{"http 前綴直接回傳", "https://media.example.com", "thumb", "https://lh3.google.com/avatar.jpg", "https://lh3.google.com/avatar.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildImageURL(tt.baseURL, tt.preset, tt.objectKey)
			if got != tt.want {
				t.Errorf("BuildImageURL(%q, %q, %q) = %q, want %q", tt.baseURL, tt.preset, tt.objectKey, got, tt.want)
			}
		})
	}
}

func TestValidateObjectKey(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		allowedPrefixes []string
		wantErr         bool
	}{
		{"合法 key（無白名單）", "public/ai-style/123/x.jpg", nil, false},
		{"合法 key（符合白名單）", "public/ai-style/123/x.jpg", []string{"public/"}, false},
		{"空字串拒絕", "", []string{"public/"}, true},
		{"https 完整 URL 拒絕", "https://test-media.example.com/public/x.jpg", []string{"public/"}, true},
		{"http 完整 URL 拒絕", "http://evil.com/x.jpg", nil, true},
		{"路徑穿越 .. 拒絕", "public/../secret/x.jpg", []string{"public/"}, true},
		{"前綴不符白名單拒絕", "tmp/staging/x.jpg", []string{"public/"}, true},
		{"多前綴白名單命中其一", "defaults/logo.png", []string{"public/", "defaults/"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateObjectKey(tt.key, tt.allowedPrefixes...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateObjectKey(%q, %v) err=%v, wantErr=%v", tt.key, tt.allowedPrefixes, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeToObjectKey(t *testing.T) {
	bases := []string{"https://test-media.example.com", "https://media.example.com"}
	prefixes := []string{"public/"}
	tests := []struct {
		name     string
		value    string
		bases    []string
		prefixes []string
		want     string
		wantErr  bool
	}{
		{"空字串回空（未提供）", "", bases, prefixes, "", false},
		{"裸 objectKey 原樣", "public/banner/1/20260717/x.jpg", bases, prefixes, "public/banner/1/20260717/x.jpg", false},
		{"CDN URL 剝殼還原 key", "https://test-media.example.com/public/banner/1/20260717/x.jpg", bases, prefixes, "public/banner/1/20260717/x.jpg", false},
		{"MediaBaseURL(media 域名) 剝殼還原 key", "https://media.example.com/public/banner/1/20260717/x.jpg", bases, prefixes, "public/banner/1/20260717/x.jpg", false},
		{"Image Gateway /i/original/ 剝殼還原 key", "https://media.example.com/i/original/public/banner/1/20260717/x.jpg", bases, prefixes, "public/banner/1/20260717/x.jpg", false},
		{"Image Gateway /i/thumb/ 剝殼還原 key", "https://media.example.com/i/thumb/public/banner/1/20260717/x.jpg", bases, prefixes, "public/banner/1/20260717/x.jpg", false},
		{"Image Gateway 路徑剝殼後前綴仍需符白名單", "https://media.example.com/i/card/tmp/staging/x.jpg", bases, prefixes, "", true},
		{"base 尾斜線也能剝殼", "https://test-media.example.com/public/banner/x.jpg", []string{"https://test-media.example.com/"}, prefixes, "public/banner/x.jpg", false},
		{"外來域名拒絕 400", "https://evil.com/public/x.jpg", bases, prefixes, "", true},
		{"外來域名（google 頭像也拒絕，輸入面不接受 URL 回存）", "https://lh3.google.com/a.jpg", bases, prefixes, "", true},
		{"剝殼後含 .. 拒絕", "https://test-media.example.com/public/../secret/x.jpg", bases, prefixes, "", true},
		{"裸 key 含 .. 拒絕", "public/../secret/x.jpg", bases, prefixes, "", true},
		{"裸 key 前綴不符白名單拒絕", "tmp/staging/x.jpg", bases, prefixes, "", true},
		{"剝殼後前綴不符白名單拒絕", "https://test-media.example.com/tmp/staging/x.jpg", bases, prefixes, "", true},
		{"無白名單時裸 key 通過", "whatever/x.jpg", bases, nil, "whatever/x.jpg", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeToObjectKey(tt.value, tt.bases, tt.prefixes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeToObjectKey(%q) err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NormalizeToObjectKey(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		value   string
		want    string
	}{
		{"空值回傳空字串", "https://cdn.example.com", "", ""},
		{"純 object key 不變", "https://cdn.example.com", "public/defaults/seller_logo.png", "public/defaults/seller_logo.png"},
		{"帶前綴被 strip", "https://cdn.example.com", "https://cdn.example.com/public/defaults/seller_logo.png", "public/defaults/seller_logo.png"},
		{"Image Gateway /i/original/ 帶前綴被 strip", "https://cdn.example.com", "https://cdn.example.com/i/original/public/defaults/seller_logo.png", "public/defaults/seller_logo.png"},
		{"Image Gateway /i/thumb/ 帶前綴被 strip", "https://cdn.example.com", "https://cdn.example.com/i/thumb/public/defaults/seller_logo.png", "public/defaults/seller_logo.png"},
		{"base URL 尾部有斜線也能 strip", "https://cdn.example.com/", "https://cdn.example.com/i/original/public/defaults/seller_logo.png", "public/defaults/seller_logo.png"},
		{"不同域名不會被 strip", "https://cdn.example.com", "https://other-cdn.com/public/logo.png", "https://other-cdn.com/public/logo.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripPrefix(tt.baseURL, tt.value)
			if got != tt.want {
				t.Errorf("StripPrefix(%q, %q) = %q, want %q", tt.baseURL, tt.value, got, tt.want)
			}
		})
	}
}
