package accountsuspension

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Suspended 在帳號停權的 transaction 內發出，含理由類別；帳號已停權的重複呼叫不發。通知（A8）消費它。
type Suspended struct {
	AccountID    int64
	SuspensionID uuid.UUID
	Category     policy.Category
	SuspendedAt  time.Time
}

// Unsuspended 在手動解除停權的 transaction 內發出；帳號沒有停權時不發。
type Unsuspended struct {
	AccountID    int64
	SuspensionID uuid.UUID
	Category     policy.Category
	ReleasedAt   time.Time
}
