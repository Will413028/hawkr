// Package media 是 media 模組的公開 API：圖片等靜態資產的暫存上傳、確認與刪除，擁有媒體的模組在自己的 transaction 內呼叫 Confirm 與 MarkDeleted。
package media

import "github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"

// Uploader 是上傳暫存圖片的人。
type Uploader = model.Uploader

// ErrNotSecure 表示 ReadSecure 的資產不是機敏資產。
var ErrNotSecure = model.ErrNotSecure

// ErrUploadAlreadyConfirmed 表示 Confirm（含 ConfirmFields）失敗的原因是 token 已被另一個已提交的 transaction 確認；錯誤的分類仍是 FailedPrecondition 或欄位錯誤。
// 擁有者以 request_id 去重時，遇到它要在回滾後再 replay 一次（並行的重送輸家）。
var ErrUploadAlreadyConfirmed = model.ErrUploadAlreadyConfirmed
