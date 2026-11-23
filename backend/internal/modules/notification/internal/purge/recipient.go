package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// RecipientArgs 是清除一個收件人範圍的 job。Audience 空字串是帳號的全部受眾；StoreID 為 0 是不限店（STORE 受眾以 StoreID 限定那間店）。
type RecipientArgs struct {
	AccountID int64  `json:"account_id"`
	Audience  string `json:"audience,omitempty"`
	StoreID   int64  `json:"store_id,omitempty"`
}

// Kind 是 River 的 job 種類。
func (RecipientArgs) Kind() string { return "notification.purge_recipient" }

// InsertOpts 讓同一個範圍還沒結束的 job 合併；清除冪等，重複的刪除事件不需要第二個 job。
func (RecipientArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(10) }

func (a RecipientArgs) validate() error {
	switch {
	case a.AccountID <= 0:
		return fmt.Errorf("清除範圍的帳號 %d 不合法", a.AccountID)
	case a.Audience != "" && a.Audience != string(model.AudienceBuyer) && a.Audience != string(model.AudienceStore):
		return fmt.Errorf("清除範圍的受眾 %q 不合法", a.Audience)
	case a.StoreID < 0, a.StoreID > 0 && a.Audience != string(model.AudienceStore):
		return fmt.Errorf("清除範圍的店 %d 只能搭配 STORE 受眾", a.StoreID)
	}
	return nil
}

// scope 把 job 參數轉成查詢的收件人範圍：NULL 表示不限。
func (a RecipientArgs) scope() (audience *string, storeID *int64) {
	if a.Audience != "" {
		audience = &a.Audience
	}
	if a.StoreID > 0 {
		storeID = &a.StoreID
	}
	return audience, storeID
}

// Recipient 清除一個收件人範圍：先把該收件人的意圖參數換成 '{}'、清掉投遞紀錄的送出文字（列留著，D16），再刪收件匣列（連結隨列刪除，
// 所以意圖要先處理），最後刪偏好覆寫；限定單店的範圍（StoreRetired）不刪偏好。每批一個 transaction，每一步都可重跑：已處理的列不再符合條件。
func (s *Service) Recipient(ctx context.Context, a RecipientArgs) error {
	if err := a.validate(); err != nil {
		return err
	}
	audience, storeID := a.scope()
	steps := []struct {
		name string
		run  func(q *store.Queries) (int64, error)
	}{
		{"意圖參數", func(q *store.Queries) (int64, error) {
			return q.ScrubRecipientIntentParams(ctx, store.ScrubRecipientIntentParamsParams{AccountID: a.AccountID, Audience: audience, StoreID: storeID, MaxRows: batchSize})
		}},
		{"投遞紀錄的送出文字", func(q *store.Queries) (int64, error) {
			return q.ScrubRecipientDeliveryText(ctx, store.ScrubRecipientDeliveryTextParams{AccountID: a.AccountID, Audience: audience, StoreID: storeID, MaxRows: batchSize})
		}},
		{"收件匣", func(q *store.Queries) (int64, error) {
			return q.DeleteRecipientInboxItems(ctx, store.DeleteRecipientInboxItemsParams{AccountID: a.AccountID, Audience: audience, StoreID: storeID, MaxRows: batchSize})
		}},
	}
	for _, step := range steps {
		if _, err := s.batches(ctx, step.run); err != nil {
			return fmt.Errorf("清除%s：%w", step.name, err)
		}
	}
	if a.StoreID > 0 {
		// 偏好是 (帳號, 受眾) 的、沒有店別；一間店退場時，同帳號可能已重開另一間店，整組 STORE 偏好不能刪。
		return nil
	}
	err := store.New(s.d.DB.Conn(ctx)).DeleteAccountPreferenceOverrides(ctx, store.DeleteAccountPreferenceOverridesParams{AccountID: a.AccountID, Audience: audience})
	if err != nil {
		return fmt.Errorf("清除偏好：%w", err)
	}
	return nil
}

type recipientWorker struct {
	river.WorkerDefaults[RecipientArgs]
	svc *Service
}

// Work 清除一個收件人範圍；失敗由 River 重試（冪等）。
func (w *recipientWorker) Work(ctx context.Context, job *river.Job[RecipientArgs]) error {
	return w.svc.Recipient(ctx, job.Args)
}

// Timeout 是一次清除的時間上限；超過時 job 重試，從剩下的列接著做。
func (w *recipientWorker) Timeout(*river.Job[RecipientArgs]) time.Duration { return 5 * time.Minute }
