// Package contentkind 是內容下架對象的種類代碼，seller（措施的對象）、social（下架複本）與 httpapi 共用同一份值。
package contentkind

// Kind 是一則社群內容的種類，也是 seller.measure_contents.content_kind 與 seller.pub_content_takedowns() 的儲存值。
type Kind string

const (
	Post       Kind = "POST"
	Comment    Kind = "COMMENT"
	ForumEntry Kind = "FORUM_ENTRY"
	// ChatMessage 是店家寄出的一則聊天訊息（im.messages.id）。
	ChatMessage Kind = "CHAT_MESSAGE"
	// ReviewReply 是店家對評價的回覆（review.reviews.id，一則評價至多一則回覆）。
	ReviewReply Kind = "REVIEW_REPLY"
)

// Values 回傳全部種類。
func Values() []Kind { return []Kind{Post, Comment, ForumEntry, ChatMessage, ReviewReply} }

// Valid 回傳 k 是否為已知種類。
func (k Kind) Valid() bool {
	switch k {
	case Post, Comment, ForumEntry, ChatMessage, ReviewReply:
		return true
	}
	return false
}

// Social 回傳 k 是否為社群內容（social 擁有並寫下架複本）；其他種類由 im、review 在讀取時經 seller.pub_content_takedowns() 判斷。
func (k Kind) Social() bool { return k == Post || k == Comment || k == ForumEntry }
