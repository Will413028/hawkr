// services/ai-service/internal/procurement/usecase/graph_runner_provider.go
package usecase

import (
	"fmt"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
)

// NewProcurementGraphRunner 把 NewProcurementGraph 接成 production 用的 *runner.Runner。
// 住在 usecase 而不是 internal/agent：SlotExtractor 是本 package 私有型別，
// 在外面建 runner 得為此匯出沒有其他理由匯出的東西。sessionSvc 重用 internal/agent/di.go
// 的 Postgres-backed session.Service；ProcurementAppName 讓兩套系統的 session 列邏輯分離。
// ResumeRunner 是同一張 graph、但**關掉 AutoCreateSession** 的 runner，只給 resume
// 路徑用。獨立型別是為了讓 fx 分得出兩個 *runner.Runner，不必加 name tag。
//
// ⛔ resume 不得 AutoCreate：session 被 24 小時清理掃掉之後，AutoCreate 會靜默開一個
// 空 session，於是這次 resume 變成「新的一輪」——上游 node 重新抽取、重新解析、再開
// 一個新的 interrupt，而買家按下的答案落在一個他從沒看過的問題上。關掉之後
// runner.getOrCreateSession 直接回 Get 的錯誤，走既有的 recovery 通道。
//
// 判準必須由 runner 自己做：先查一次存在、後面仍 AutoCreate 的寫法還有 TOCTOU 窗口。
type ResumeRunner struct{ *runner.Runner }

func NewProcurementGraphRunner(
	extractor SlotExtractor, resolver cropidentity.Resolver, newID func() int64, sessionSvc session.Service, pricePreviewChecker PricePreviewChecker, narrator StepNarrator, realizer SurfaceRealizer,
) (*runner.Runner, ResumeRunner, error) {
	g, err := NewProcurementGraph(extractor, resolver, newID, pricePreviewChecker, narrator, realizer)
	if err != nil {
		return nil, ResumeRunner{}, fmt.Errorf("building procurement graph: %w", err)
	}
	// 新的一輪仍然要能建立 session（第一輪時它本來就不存在）。
	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: g, SessionService: sessionSvc, AutoCreateSession: true,
	})
	if err != nil {
		return nil, ResumeRunner{}, fmt.Errorf("building procurement graph runner: %w", err)
	}
	resume, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: g, SessionService: sessionSvc, AutoCreateSession: false,
	})
	if err != nil {
		return nil, ResumeRunner{}, fmt.Errorf("building procurement graph resume runner: %w", err)
	}
	return r, ResumeRunner{Runner: resume}, nil
}
