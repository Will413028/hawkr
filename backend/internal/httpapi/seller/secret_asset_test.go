package seller

import (
	"strings"
	"uuid"
)

var secretAssets = [...]uuid.UUID{
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a1"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a2"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a3"),
	uuid.MustParse("0192a000-0000-7000-8000-0000000000a4"),
}

func leaksSecretAsset(text string) bool {
	for _, id := range secretAssets {
		if strings.Contains(text, id.String()) {
			return true
		}
	}
	return false
}
