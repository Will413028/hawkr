// Package contentkind 是內容下架對象的種類代碼，seller（措施的對象）、social（下架複本）與 httpapi 共用同一份值。
package contentkind

// Kind 是一則社群內容的種類，也是 seller.measure_contents.content_kind 與 seller.pub_content_takedowns() 的儲存值。
type Kind string

const (
	Post       Kind = "POST"
	Comment    Kind = "COMMENT"
	ForumEntry Kind = "FORUM_ENTRY"
)

// Values 回傳全部種類。
func Values() []Kind { return []Kind{Post, Comment, ForumEntry} }

// Valid 回傳 k 是否為已知種類。
func (k Kind) Valid() bool {
	switch k {
	case Post, Comment, ForumEntry:
		return true
	}
	return false
}
