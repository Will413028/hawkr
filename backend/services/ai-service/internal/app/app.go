package app

import (
	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	buyerClient "github.com/Will413028/hawkr/ai-service/internal/client/buyer"
	categoryClient "github.com/Will413028/hawkr/ai-service/internal/client/category"
	mediaClient "github.com/Will413028/hawkr/ai-service/internal/client/media"
	orderClient "github.com/Will413028/hawkr/ai-service/internal/client/order"
	productClient "github.com/Will413028/hawkr/ai-service/internal/client/product"
	sellerClient "github.com/Will413028/hawkr/ai-service/internal/client/seller"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	grpcLayer "github.com/Will413028/hawkr/ai-service/internal/grpc"
	"github.com/Will413028/hawkr/ai-service/internal/infrastructure"
	"github.com/Will413028/hawkr/ai-service/internal/procurement"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(modules()...)
}

// modules is shared by production and TestFxGraph_Valid so the graph test
// cannot drift away from the application that actually starts in deployment.
func modules() []fx.Option {
	return []fx.Option{
		config.Module,
		infrastructure.Module,
		categoryClient.Module,              // Provides *category.Client（分類樹）
		orderClient.Module,                 // Provides usecase.OrderClient
		productClient.Module,               // Provides usecase.ProductClient
		sellerClient.Module,                // Provides usecase.SellerClient
		buyerClient.Module,                 // Provides buyer.Client（買家會員 entitlement／quota）
		mediaClient.Module,                 // Provides media.Client（AI 風格縮圖 finalize）
		agentpkg.Module,                    // Provides agent.Agents + agent.Runners + session.Service（adk-go）
		usecase.Module,                     // Provides UseCases（通用聊天/推薦）
		fx.Provide(cache.NewCategoryIndex), // Provides *cache.CategoryIndex（分類 in-memory index）
		procurement.Module,                 // Hawkr 幫找採購助手（procurement 域，含 repository/usecase/handler 三層）
		grpcLayer.Module,                   // gRPC handlers
	}
}
