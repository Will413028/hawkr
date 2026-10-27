package modelregistry

import (
	"strings"
	"testing"
)

func TestNewR2Downloader_RejectsEmptyFields(t *testing.T) {
	cases := []struct {
		name                                            string
		accountID, accessKeyID, secretAccessKey, bucket string
	}{
		{"empty accountID", "", "key", "secret", "bucket"},
		{"empty accessKeyID", "acct", "", "secret", "bucket"},
		{"empty secretAccessKey", "acct", "key", "", "bucket"},
		{"empty bucket", "acct", "key", "secret", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewR2Downloader(t.Context(), tc.accountID, tc.accessKeyID, tc.secretAccessKey, tc.bucket)
			if err == nil {
				t.Fatal("expected error for empty required field")
			}
		})
	}
}

func TestNewR2Downloader_BuildsAccountScopedEndpoint(t *testing.T) {
	dl, err := NewR2Downloader(t.Context(), "myaccount123", "key", "secret", "mybucket")
	if err != nil {
		t.Fatalf("NewR2Downloader: %v", err)
	}
	if !strings.Contains(dl.endpoint, "myaccount123.r2.cloudflarestorage.com") {
		t.Errorf("endpoint = %q, want it to contain the account-scoped R2 host", dl.endpoint)
	}
	if dl.bucket != "mybucket" {
		t.Errorf("bucket = %q, want %q", dl.bucket, "mybucket")
	}
}
