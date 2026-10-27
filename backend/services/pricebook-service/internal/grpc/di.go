package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.PricebookService_ServiceDesc.ServiceName)
}

// registerPricebookService runs during fx graph construction, before the
// shared gRPC server's OnStart hook can accept connections.
func registerPricebookService(server *grpclib.Server, handler *PricebookServer) {
	pb.RegisterPricebookServiceServer(server, handler)
}

func markPricebookReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpclib.Server,
	_ *PricebookServer,
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

// Module registers PricebookServer against the shared final gRPC bootstrap.
//
// usecase.Module provides the concrete *usecase.ObservedPrice,
// *usecase.EstimatedItemPrice, *usecase.ForecastCurve,
// *usecase.SearchCropCatalog, (Task 8) *usecase.TrendSummary, (issue
// #2248) *usecase.SpatialSurface, (issue #2249 Slice C)
// *usecase.RegionalComparison, (issue #2786) *usecase.ResolveCategoryMapping,
// *usecase.UpsertCategoryMapping, *usecase.ListCategoryMappingWorklist and
// (Task 10) *usecase.GetCategoryPriceRollup; the eleven fx.Provide calls
// below each bridge one into the narrow interface
// NewPricebookServer depends on (see handler.go's doc comments) — dig
// matches providers by exact type, so an interface-typed parameter needs an
// explicit provider returning that interface, it won't automatically
// accept a concrete type that happens to implement it. mapping.Seed
// doesn't need a bridge — NewPricebookServer takes it as a concrete value
// type, which fx already provides via usecase.Module's provideMappingSeed.
var Module = fx.Module("grpc",
	fx.Provide(func(uc *usecase.ObservedPrice) observedPriceUC { return uc }),
	fx.Provide(func(uc *usecase.EstimatedItemPrice) estimatedItemPriceUC { return uc }),
	fx.Provide(func(uc *usecase.ForecastCurve) forecastCurveUC { return uc }),
	fx.Provide(func(uc *usecase.SearchCropCatalog) searchCropCatalogUC { return uc }),
	fx.Provide(func(uc *usecase.SearchPriceableEntities) searchPriceableEntitiesUC { return uc }),
	fx.Provide(func(uc *usecase.TrendSummary) trendSummaryUC { return uc }),
	fx.Provide(func(uc *usecase.SpatialSurface) spatialSurfaceUC { return uc }),
	fx.Provide(func(uc *usecase.RegionalComparison) regionalComparisonUC { return uc }),
	fx.Provide(func(uc *usecase.ResolveCategoryMapping) resolveCategoryMappingUC { return uc }),
	fx.Provide(func(uc *usecase.UpsertCategoryMapping) upsertCategoryMappingUC { return uc }),
	fx.Provide(func(uc *usecase.ListCategoryMappingWorklist) listCategoryMappingWorklistUC { return uc }),
	fx.Provide(func(uc *usecase.GetCategoryPriceRollup) getCategoryPriceRollupUC { return uc }),
	fx.Provide(func(uc *usecase.CommodityForecast) commodityForecastUC { return uc }),
	fx.Provide(newMarketPricebookServer),
	fx.Provide(newHealthController),
	fx.Invoke(registerPricebookService),
	fx.Invoke(markPricebookReady),
)

// Inject personalized usecases at construction without changing the legacy test constructor.
func newMarketPricebookServer(uc observedPriceUC, estimatedUC estimatedItemPriceUC, curveUC forecastCurveUC, catalogUC searchCropCatalogUC, seed mapping.Seed, trendUC trendSummaryUC, spatialUC spatialSurfaceUC, regionalUC regionalComparisonUC, resolveUC resolveCategoryMappingUC, upsertUC upsertCategoryMappingUC, worklistUC listCategoryMappingWorklistUC, rollupUC getCategoryPriceRollupUC, commodityUC commodityForecastUC, priceableUC searchPriceableEntitiesUC, overview *usecase.MarketOverview, watch *usecase.MarketWatch) *PricebookServer {
	server := NewPricebookServer(uc, estimatedUC, curveUC, catalogUC, seed, trendUC, spatialUC, regionalUC, resolveUC, upsertUC, worklistUC, rollupUC, commodityUC, priceableUC)
	server.overviewUC = overview
	server.watchUC = watch
	return server
}
