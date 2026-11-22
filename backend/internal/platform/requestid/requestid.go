// Package requestid 是 request_id 的格式判定：契約上的 request_id 一律是 RFC 4122 的 UUIDv4。
package requestid

import "uuid"

// IsV4 回報 id 是否為 UUIDv4（版本位 4、variant 10）；零值與其他版本一律不是。
func IsV4(id uuid.UUID) bool {
	return id[6]>>4 == 4 && id[8]>>6 == 0b10
}
