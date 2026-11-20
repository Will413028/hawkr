package im

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
)

// SetLiveHooks 設定即時通道的測試縫：afterRegister 在 Subscribe 登記之後、補洞之前；afterBackfill 在第一輪補洞之後、Ready 之前。
// 必須在任何 Subscribe 之前呼叫。
func SetLiveHooks(m *Module, afterRegister, afterBackfill func()) {
	m.hub.AfterRegister, m.hub.AfterBackfill = afterRegister, afterBackfill
}

// LiveReceived 是這個模組的監聽端收到的通知總數。
func LiveReceived(m *Module) int64 { return m.hub.Received() }

// RunWithGatedDial 是 Run，但第一次撥號擋在 gate 上：dialing 在撥號開始（啟動時的資料庫時間已經讀完）時收到一個訊號。
func RunWithGatedDial(ctx context.Context, m *Module, dialing chan<- struct{}, gate <-chan struct{}) error {
	cfg := m.listenCfg
	if cfg == nil {
		cfg = m.pool.Pool().Config().ConnConfig
	}
	real := live.DialPG(cfg)
	return m.hub.Run(ctx, func(ctx context.Context) (live.Conn, error) {
		select {
		case dialing <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return real(ctx)
	})
}
