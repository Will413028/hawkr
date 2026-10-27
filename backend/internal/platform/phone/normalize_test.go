package phone

import "testing"

func TestNormalizeToLocal(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		valid    bool
	}{{"+886 0912-345-678", "0912345678", true}, {"8860912345678", "0912345678", true}, {"+886 (02) 2345-6789", "0223456789", true}, {"+886 2 2345-6789", "0223456789", true}, {"+886 912-345-678", "0912345678", true}, {"886912345678", "0912345678", true}, {"(02) 2345-6789", "0223456789", true}, {"0912345678", "0912345678", true}, {"", "", false}, {"+12025550123", "+12025550123", false}, {"09123x5678", "09123x5678", false}, {"09123", "09123", false}} {
		t.Run(tt.in, func(t *testing.T) {
			got := NormalizeToLocal(tt.in)
			if got != tt.want || Valid(got) != tt.valid {
				t.Fatalf("got %q valid=%v", got, Valid(got))
			}
		})
	}
}

func TestFormatWithAreaCode(t *testing.T) {
	for in, want := range map[string]string{
		"0229010572":      "02-29010572",
		"0298887777#123":  "02-98887777#123",
		"033123456":       "03-3123456",
		"0372123456":      "037-2123456",
		"0492123456":      "049-2123456",
		"0892123456":      "089-2123456",
		"082312345":       "082-312345",
		"0826123456":      "0826-123456",
		"0800123456":      "0800-123456",
		"0809123456":      "0809-123456",
		"0800123456#12":   "0800-123456#12",
		"0812345678":      "08-12345678",
		"0836123456":      "0836-123456",
		"0912345678":      "09-12345678",
		"0422222222#9":    "04-22222222#9",
		"062345678":       "06-2345678",
		"0712345678#1234": "07-12345678#1234",
	} {
		if got, ok := FormatWithAreaCode(in); !ok || got != want {
			t.Errorf("FormatWithAreaCode(%q) = %q, %v，要 %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "0129010572", "02", "02123", "0212345678901", "0298887777#", "0298887777#12a", "0298887777#1234567", "2298887777"} {
		if got, ok := FormatWithAreaCode(in); ok {
			t.Errorf("FormatWithAreaCode(%q) = %q 應該失敗", in, got)
		}
	}
}
