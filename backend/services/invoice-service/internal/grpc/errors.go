package grpc

import (
	"errors"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// 型別邊界的三個解析錯誤。具名而不是每次 fmt.Errorf：呼叫端把它們包進
// codes.InvalidArgument 的訊息裡，訊息一致才看得出是同一類契約違規。
var (
	errEmptyID       = errors.New("must not be empty")
	errNonPositiveID = errors.New("must be a positive integer")
	errMissingAmount = errors.New("amount breakdown is required")
)

// newBizErr 造一個帶業務碼的 DomainError（測試與 fake 用；正式路徑的來源是 repository
// 的 notFound()，本函式只是讓 fake 能回出同形狀的錯誤）。
func newBizErr(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}
