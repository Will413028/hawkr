package reply

import (
	"fmt"
	"uuid"
)

// categoryID 是測試用的確定性分類 ID：同一個數字永遠得到同一個 UUID，且不是零值。
func categoryID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}
