package seller

import (
	"strings"
	"uuid"
)

// secretAssets 是治理佐證的 asset ID：只給擁有者讀取與紀錄，不得出現在任何回應。
var secretAssets = [...]uuid.UUID{
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a1"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a2"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a3"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a4"),
}

// leaksSecretAsset 回報 text 是否含任何 secretAssets 的文字。
func leaksSecretAsset(text string) bool {
	for _, id := range secretAssets {
		if strings.Contains(text, id.String()) {
			return true
		}
	}
	return false
}
