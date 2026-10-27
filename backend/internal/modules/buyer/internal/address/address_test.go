package address

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
)

func TestRecipientName(t *testing.T) {
	for _, tt := range []struct {
		name string
		ok   bool
	}{{"王明", true}, {"John", true}, {"王", false}, {"Joe", false}, {strings.Repeat("王", 15), true}, {strings.Repeat("王", 16), false}, {strings.Repeat("a", 30), true}, {strings.Repeat("a", 31), false}, {"  ", false}} {
		input := model.AddressInput{
			ReceiverName:  tt.name,
			ReceiverPhone: "+886912345678",
			TownCode:      "63000050",
			AddressDetail: "測試路",
		}
		err := Validate(&input)
		if (err == nil) != tt.ok {
			t.Errorf("%q: %v", tt.name, err)
		}
		if err == nil && input.ReceiverPhone != "0912345678" {
			t.Fatal(input.ReceiverPhone)
		}
	}
}

func TestTownCodeValidation(t *testing.T) {
	for _, tt := range []struct {
		code string
		ok   bool
	}{
		{"63000050", true}, // 臺北市中正區
		{"65000010", true}, // 新北市板橋區
		{"", false},
		{"invalid", false},
		{"00000000", false},
		{"1234567", false},   // 7 碼
		{"123456789", false}, // 9 碼
	} {
		input := model.AddressInput{
			ReceiverName:  "王小明",
			ReceiverPhone: "0912345678",
			TownCode:      tt.code,
			AddressDetail: "測試路 1 號",
		}
		err := Validate(&input)
		if (err == nil) != tt.ok {
			t.Errorf("code %q: got err %v, want ok %v", tt.code, err, tt.ok)
		}
	}
}

func TestAddressDetailBounds(t *testing.T) {
	for _, tt := range []struct {
		detail string
		ok     bool
	}{
		{"a", true},
		{"中", true},
		{"松仁路 100 號", true},
		{strings.Repeat("字", 255), true},
		{strings.Repeat("字", 256), false},
		{"", false},
		{"   ", false},
	} {
		input := model.AddressInput{
			ReceiverName:  "王小明",
			ReceiverPhone: "0912345678",
			TownCode:      "63000050",
			AddressDetail: tt.detail,
		}
		err := Validate(&input)
		if (err == nil) != tt.ok {
			t.Errorf("detail len %d: got err %v, want ok %v", len(tt.detail), err, tt.ok)
		}
	}
}
