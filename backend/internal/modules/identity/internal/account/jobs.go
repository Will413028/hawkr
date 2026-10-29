package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

// AddWorkers 登記本模組的 job worker，給 worker 角色用。
func (m *Service) AddWorkers(w *river.Workers) {
	river.AddWorker(w, &clerkUserWorker{m: m})
}

// clerkUserAction 是對 Clerk 使用者做的事。
type clerkUserAction string

const clerkUserActionDelete clerkUserAction = "delete"

// clerkUserMaxAttempts：River 預設的退避（attempt^4 秒）下，第 10 次大約在四小時後；都失敗時 job 留在 discarded，
// 要人工處理。帳號在本模組已經刪除，失敗只影響 Clerk 那邊的使用者。
const clerkUserMaxAttempts = 10

// clerkUserArgs 刪除一個 Clerk 使用者（刪帳的 transaction 內排入）。
type clerkUserArgs struct {
	Action      clerkUserAction `json:"action"`
	ClerkUserID string          `json:"clerk_user_id"`
}

func (clerkUserArgs) Kind() string { return "identity.clerk_user" }

// enqueueClerkUserJob 在呼叫方的 transaction 內排入 job：rollback 時 job 也不存在。
func (m *Service) enqueueClerkUserJob(ctx context.Context, action clerkUserAction, clerkUserID string) error {
	args := clerkUserArgs{Action: action, ClerkUserID: clerkUserID}
	if err := m.d.Jobs.Enqueue(ctx, args, &river.InsertOpts{MaxAttempts: clerkUserMaxAttempts}); err != nil {
		return fmt.Errorf("identity：排入 Clerk 使用者的 %s：%w", action, err)
	}
	return nil
}

type clerkUserWorker struct {
	river.WorkerDefaults[clerkUserArgs]
	m *Service
}

// Work 呼叫 Clerk 的 Backend API。使用者不屬於任何設定的 application（例如本機 seed 的 unlinked: 帳號）時不做任何事；
// Clerk 拒絕（重試不會成功）時取消。
func (w *clerkUserWorker) Work(ctx context.Context, job *river.Job[clerkUserArgs]) error {
	id := job.Args.ClerkUserID
	c := clerkOwning(w.m.d.Customers, id)
	if c == nil {
		w.m.d.Logger.WarnContext(ctx, "identity：使用者不屬於任何 Clerk application，略過", "action", job.Args.Action, "clerk_user_id", id)
		return nil
	}
	var err error
	switch job.Args.Action {
	case clerkUserActionDelete:
		err = c.DeleteUser(ctx, id)
	default:
		return river.JobCancel(fmt.Errorf("identity：未知的 Clerk 使用者動作 %q", job.Args.Action))
	}
	if errors.Is(err, model.ErrRejected) {
		return river.JobCancel(fmt.Errorf("identity：Clerk 拒絕 %s：%w", job.Args.Action, err))
	}
	if err != nil {
		return fmt.Errorf("identity：Clerk 使用者的 %s：%w", job.Args.Action, err)
	}
	return nil
}

// clerkOwning 回傳發出這個使用者 ID 的 application。
func clerkOwning(apps []model.Clerk, userID string) model.Clerk {
	for _, c := range apps {
		if c.OwnsUser(userID) {
			return c
		}
	}
	return nil
}
