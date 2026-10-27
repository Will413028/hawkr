package payuni

import "testing"

func TestMerchantStoreURL(t *testing.T) {
	tests := []struct {
		name       string
		webBaseURL string
		sellerID   string
		want       string
	}{
		{
			name:       "default prod domain uses /sellers/ path",
			webBaseURL: "https://example.com",
			sellerID:   "123456789",
			want:       "https://example.com/sellers/123456789",
		},
		{
			name:       "trailing slash is trimmed",
			webBaseURL: "https://example.com/",
			sellerID:   "123456789",
			want:       "https://example.com/sellers/123456789",
		},
		{
			name:       "stage override",
			webBaseURL: "https://stage.example.com",
			sellerID:   "987654321",
			want:       "https://stage.example.com/sellers/987654321",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := merchantStoreURL(tt.webBaseURL, tt.sellerID); got != tt.want {
				t.Errorf("merchantStoreURL(%q, %q) = %q, want %q", tt.webBaseURL, tt.sellerID, got, tt.want)
			}
		})
	}
}

func TestResolveMerURL(t *testing.T) {
	tests := []struct {
		name       string
		websiteURL string
		webBaseURL string
		storeID    string
		want       string
	}{
		{
			name:       "seller website url takes priority",
			websiteURL: "https://my-shop.example.com",
			webBaseURL: "https://example.com",
			storeID:    "123456789",
			want:       "https://my-shop.example.com",
		},
		{
			name:       "falls back to platform page when empty",
			websiteURL: "",
			webBaseURL: "https://example.com",
			storeID:    "123456789",
			want:       "https://example.com/sellers/123456789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveMerURL(tt.websiteURL, tt.webBaseURL, tt.storeID); got != tt.want {
				t.Errorf("resolveMerURL(%q, %q, %q) = %q, want %q", tt.websiteURL, tt.webBaseURL, tt.storeID, got, tt.want)
			}
		})
	}
}
