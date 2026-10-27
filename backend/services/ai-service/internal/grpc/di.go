package grpc

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const grpcMaxMsgSize = 20 * 1024 * 1024 // 20MB，圖片處理需要傳輸原圖與 base64 結果

// NewGRPCServer creates a new gRPC server and registers the AI service.
//
// Deliberately takes (handler, assistantHandler, logger) instead of an
// fx.Lifecycle-coupled params struct like pkg/server.NewGRPCServer's other
// adopters: di_test.go calls this directly (bufconn, no real listener,
// no fx) to end-to-end verify UnaryMethodPolicyInterceptor/
// StreamMethodPolicyInterceptor are actually wired — see that test's
// comment. Interceptor chain construction still comes from
// pkgserver.BuildServerOptions (shared with every other service). Runtime
// listener binding is handled by pkgserver.NewGRPCServer; this direct
// constructor remains a no-lifecycle bufconn test seam only.
func NewGRPCServer(handler *AIGrpcServer, assistantHandler *AIAssistantGrpcServer, logger *zap.Logger) *grpc.Server {
	cfg := newGRPCServerConfig(&config.Config{})
	s := grpc.NewServer(pkgserver.BuildServerOptions(cfg, logger)...)
	registerAIImageService(s, handler)
	registerAIAdminService(s, handler)
	registerRecommendationService(s, handler)
	pb.RegisterAIAssistantServiceServer(s, assistantHandler)
	reflection.Register(s)
	return s
}

func newGRPCServerConfig(cfg *config.Config) pkgserver.ServerConfig {
	return pkgserver.ServerConfig{
		ServerConfig: pkgconfig.ServerConfig{
			GRPCPort: cfg.Server.GRPCPort,
		},
		HealthServiceNames:       aiHealthServiceNames(),
		Env:                      cfg.Env,
		EnableStreamInterceptors: true,
		MaxRecvMsgSize:           grpcMaxMsgSize,
		MaxSendMsgSize:           grpcMaxMsgSize,
		ExtraUnaryInterceptors:   []grpc.UnaryServerInterceptor{UnaryMethodPolicyInterceptor()},
		ExtraStreamInterceptors:  []grpc.StreamServerInterceptor{StreamMethodPolicyInterceptor()},
	}
}

// aiHealthServiceNames 是這個 process 對外提供的全部 gRPC service 名稱，供
// grpc.health.v1 的具名 health check 用。#3519 之後 legacy AIService 變成三個
// （AIImageService／AIAdminService／RecommendationService），所以這裡是五個。
//
// 🔴 k8s readiness probe（k8s/values/ai-service.yaml 的 probes.readiness.grpcService）
// 與 docker-compose-local.yml 的 healthcheck 各自寫死其中一個名字；這份清單少一個
// 名字，探針就會拿到 NOT_FOUND ⇒ pod 永遠 NotReady。改名時兩邊要一起改。
func aiHealthServiceNames() []string {
	return []string{
		pb.AIImageService_ServiceDesc.ServiceName,
		pb.AIAdminService_ServiceDesc.ServiceName,
		pb.RecommendationService_ServiceDesc.ServiceName,
		pb.AIAssistantService_ServiceDesc.ServiceName,
		pb.AIProcurementService_ServiceDesc.ServiceName,
	}
}

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(aiHealthServiceNames()...)
}

// #3519：legacy AIService 依授權軸拆成三個 service，但實作仍是同一個
// AIGrpcServer（拆的是契約表面，不是 struct——見 handler.go 的型別註解），
// 所以這裡是三次註冊、同一個 handler。
func registerAIImageService(s *grpc.Server, handler *AIGrpcServer) {
	pb.RegisterAIImageServiceServer(s, handler)
}

func registerAIAdminService(s *grpc.Server, handler *AIGrpcServer) {
	pb.RegisterAIAdminServiceServer(s, handler)
}

func registerRecommendationService(s *grpc.Server, handler *AIGrpcServer) {
	pb.RegisterRecommendationServiceServer(s, handler)
}

func registerAIAssistantService(s *grpc.Server, handler *AIAssistantGrpcServer) {
	pb.RegisterAIAssistantServiceServer(s, handler)
}

// registerAIProcurementService 把 AIProcurementGrpcServer 掛上既有的 *grpc.Server。
// 刻意不把 procurementHandler 加進 NewGRPCServer 的參數列——那樣的話 di_test.go
// 裡直接呼叫 NewGRPCServer(handler, assistantHandler, logger) 的既有測試（bufconn
// 端到端驗證 method-policy interceptor 是否真的接上）就要跟著改簽章，讓一個
// procurement RPC 的新增牽動一個完全不相關領域（AIAssistantService policy
// regression test）的既有測試檔案。用 fx.Invoke 在 *grpc.Server 建好之後另外註冊
// 一次，兩者互不影響：fx 的 invoke 函式在 fx.New() 建圖階段就會執行（不是等
// app.Start()），一定早於 pkgserver.NewGRPCServer 那個 OnStart hook 真正
// 呼叫 server.Serve 的時間點，註冊順序不會有競態。
func registerAIProcurementService(s *grpc.Server, procurementHandler *AIProcurementGrpcServer) {
	pb.RegisterAIProcurementServiceServer(s, procurementHandler)
}

func markAIReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpc.Server,
	_ *AIGrpcServer,
	_ *AIAssistantGrpcServer,
	_ *AIProcurementGrpcServer,
) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			health.MarkReady()
			return nil
		},
		OnStop: func(context.Context) error {
			health.MarkNotReady()
			return nil
		},
	})
}

// Module provides the gRPC layer dependencies.
var Module = fx.Module("grpc",
	fx.Provide(NewAIGrpcServer),
	fx.Provide(NewAIAssistantGrpcServer),
	// NewAIProcurementGrpcServer 的 12 個依賴（turn/snapshot/candidate/selection/
	// answer/rewind/crop/quoteBatch/withdraw/candidates/conversation/confirm）
	// 全部已經由 procurement.Module（見 internal/procurement/module.go）以同一個
	// instance 提供給 procurement transport 用，且沒有掛 fx.Private——procurement.Module
	// 與本 module 是同一個 fx.New() 底下的 sibling module，fx 會直接把同一批
	// instance 注入這裡，不需要在這裡另外 construct 一次。RunStore（Task 11，供
	// WatchRunEvents 使用）同一個道理：procurement.Module 以
	// `func(r *repository.Repo) grpc.RunStore { return r }` 提供。
	fx.Provide(NewAIProcurementGrpcServer),
	// DurableRunDeps 的兩個成員都已經在圖裡：usecase.ResumeRunStore（Task 14b
	// 放寬，原本是 usecase.RunClaimStore）與 *usecase.RunDispatcher 由
	// procurement.Module 提供（見 internal/procurement/module.go）。這裡只是把
	// 它們收成一個具名參數物件，避免 NewAIProcurementGrpcServer 的位置參數再長
	// 兩個。Config 欄位已由 T12 procurement-graph-cutover 計畫 Task 6a 移除
	// （SubmitTurn／SubmitTurnStream 兩個 gRPC 入口改為無條件走 graph，flag-gated
	// enabled() 判斷已無呼叫者），這裡不再需要 *config.Config 依賴。
	fx.Provide(func(runStore usecase.ResumeRunStore, dispatcher *usecase.RunDispatcher) DurableRunDeps {
		return DurableRunDeps{RunStore: runStore, Dispatcher: dispatcher}
	}),
	// GraphControllerDeps 是 DurableRunDeps 的直接類比，同一個組裝理由——只是
	// 多收一個 *runner.Runner（Task 5 的 usecase.NewProcurementGraphRunner 提供，
	// 見 internal/procurement/module.go）。這裡的 runStore 沿用同一個
	// usecase.ResumeRunStore instance：ResumeRunStore 內嵌 RunClaimStore，滿足
	// 前者的值天然滿足 GraphControllerDeps.RunStore 要的後者這個更窄介面。
	//
	// usecase.GraphControllerDeps（Sub-project 3b, Task 5）：型別本體移到
	// usecase package——本檔不再維護一份平行 struct，只是組裝它。CropResolver
	// 是新加的第九個欄位，供 Task 8 的 resumeGraphTurn 用；internal/procurement/
	// module.go 已經以 `func(uc *cropidentity.DisambiguationUseCase) cropidentity.Resolver { return uc }`
	// 提供，cropidentity.Resolver 因此已可直接 fx 解析，不需要另外改 module.go。
	//
	// Config 欄位已由 T12 procurement-graph-cutover 計畫 Task 7 移除——
	// ClaimAndDispatchTurnViaGraph／resumeGraphTurn 從未讀過 deps.Config（唯一
	// 讀者是 Task 4 已刪除的 PROCUREMENT_LLM_CONTROLLER_ENABLED flag 檢查），這裡
	// 也不再需要 *config.Config 依賴，理由同上面 DurableRunDeps 那份 Task 6a 註解。
	//
	// Narrator 是第十個欄位：它由 2026-09-03 的 `77b8ffd9e` 隨兩碼敘述一起加進
	// GraphControllerDeps，但這裡當時沒有跟著補，於是消歧暫停那條分支
	// （graphTurnExecutor 自己 compose 的那一輪）拿到的一直是 nil narrator，
	// UNDERSTAND_INPUT／PICK_NEXT_QUESTION 兩碼的 Step.Text 一直是空字串。
	// 走 terminal node 的正常輪次不經過這裡（narrator 由 graph 構造時閉包進去），
	// 所以症狀只出現在消歧那條路上。graph_controller_deps_wiring_test.go 守這件事。
	//
	// Realizer 是第十一個欄位（#2549 Phase 3, Task 8）：跟 Narrator 同一個分工
	// ——走 terminal node 的正常輪次不經過這裡（realizer 由 graph 構造時閉包進
	// 去，見 usecase.NewProcurementGraph），這個欄位只服務消歧暫停那條分支。
	// usecase.NewSurfaceRealizer 已由 procurement.Module 提供（Task 7），
	// usecase.SurfaceRealizer 因此已可直接 fx 解析，不需要另外改 module.go。
	// 這次刻意跟 Narrator 一起補進同一個 composite literal，不重蹈上面那段
	// 註解記的漏接。
	// ResumeRunner 是第十二個欄位：同一張 graph、關掉 AutoCreateSession，只給
	// resume 路徑。checkpoint 被 24 小時清理掃掉之後，AutoCreate 會靜默開一個空
	// session 把 resume 跑成新的一輪（上游重抽取、再開一個 interrupt），買家的
	// 答案落在他沒看過的問題上。usecase.NewProcurementGraphRunner 一次回兩個。
	fx.Provide(func(runStore usecase.ResumeRunStore, revStore usecase.RevisionStore, dispatcher *usecase.RunDispatcher, r *runner.Runner, resumeRunner usecase.ResumeRunner, rfqConfirmationReader usecase.RfqConfirmationReader, entitlement usecase.EntitlementClient, quota usecase.QuotaReserver, cropResolver cropidentity.Resolver, narrator usecase.StepNarrator, realizer usecase.SurfaceRealizer) usecase.GraphControllerDeps {
		return usecase.GraphControllerDeps{
			RunStore: runStore, RevisionStore: revStore, Dispatcher: dispatcher, Runner: r,
			ResumeRunner:          resumeRunner,
			RfqConfirmationReader: rfqConfirmationReader, Entitlement: entitlement, Quota: quota,
			CropResolver: cropResolver, Narrator: narrator, Realizer: realizer,
		}
	}),
	fx.Provide(newGRPCServerConfig),
	fx.Provide(newHealthController),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Invoke(registerAIImageService),
	fx.Invoke(registerAIAdminService),
	fx.Invoke(registerRecommendationService),
	fx.Invoke(registerAIAssistantService),
	fx.Invoke(registerAIProcurementService),
	fx.Invoke(markAIReady),
)
