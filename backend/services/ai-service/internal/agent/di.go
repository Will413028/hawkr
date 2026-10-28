package agent

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	pkgdb "github.com/Will413028/hawkr/go-pkg/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/session/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func provideToolDeps(
	orders usecase.OrderClient,
	products usecase.ProductClient,
	categoryIndex *cache.CategoryIndex,
	candidateLeadSearcher usecase.CandidateLeadSearcher,
) *ToolDeps {
	return &ToolDeps{
		OrderClient:           orders,
		ProductClient:         products,
		CategoryIndex:         categoryIndex,
		CandidateLeadSearcher: candidateLeadSearcher,
	}
}

func sessionDSN(cfg *config.Config) string {
	return pkgdb.BuildDSNKeyword(pkgdb.DBConfig{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Name:     cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
}

func provideSessionService(lc fx.Lifecycle, cfg *config.Config) (session.Service, error) {
	dsn := sessionDSN(cfg)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, fmt.Errorf("開啟 session 連線池失敗: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("取得 session 連線池 sql.DB 失敗: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return sqlDB.Close()
		},
	})

	svc, err := database.NewSessionService(postgres.New(postgres.Config{Conn: sqlDB}))
	if err != nil {
		return nil, fmt.Errorf("建立 session database service 失敗: %w", err)
	}
	return svc, nil
}

type Runners struct {
	Specialists map[string]*runner.Runner

	Coordinators map[string]*runner.Runner

	OrderPresenter  *runner.Runner
	ProductAutofill *runner.Runner
	ProductListing  *runner.Runner
}

func newRunner(ag agent.Agent, sessionSvc session.Service) (*runner.Runner, error) {
	return runner.New(runner.Config{
		AppName:           string(AppNameAssistant),
		Agent:             ag,
		SessionService:    sessionSvc,
		AutoCreateSession: true,
	})
}

func provideRunners(agents *Agents, sessionSvc session.Service) (*Runners, error) {
	inMemSvc := session.InMemoryService()

	specialistRunners := make(map[string]*runner.Runner)

	buildButtonRunners := func(role string, specs []specialistSpec) error {
		for _, spec := range specs {
			if spec.ButtonKey == "" {
				continue
			}
			key := role + ":" + spec.Key
			r, err := newRunner(agents.Specialists[key], sessionSvc)
			if err != nil {
				return err
			}
			specialistRunners[key] = r
		}
		return nil
	}
	if err := buildButtonRunners("seller", sellerSpecialistSpecs()); err != nil {
		return nil, err
	}
	if err := buildButtonRunners("buyer", buyerSpecialistSpecs()); err != nil {
		return nil, err
	}

	coordRunners := make(map[string]*runner.Runner)
	for role, coord := range agents.Coordinators {
		r, err := newRunner(coord, sessionSvc)
		if err != nil {
			return nil, err
		}
		coordRunners[role] = r
	}

	orderPresenterR, err := newRunner(agents.OrderPresenter, sessionSvc)
	if err != nil {
		return nil, err
	}
	productAutofillR, err := newRunner(agents.ProductAutofill, inMemSvc)
	if err != nil {
		return nil, err
	}
	productListingR, err := newRunner(agents.ProductListing, sessionSvc)
	if err != nil {
		return nil, err
	}

	return &Runners{
		Specialists:     specialistRunners,
		Coordinators:    coordRunners,
		OrderPresenter:  orderPresenterR,
		ProductAutofill: productAutofillR,
		ProductListing:  productListingR,
	}, nil
}

func registerSessionCleaner(lc fx.Lifecycle, pool *pgxpool.Pool) error {
	cleaner := newSessionCleaner(pool)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			cleaner.Start()
			return nil
		},
		OnStop: func(context.Context) error {
			cleaner.Stop()
			return nil
		},
	})
	return nil
}

var Module = fx.Module("agent",
	fx.Provide(provideToolDeps),

	fx.Provide(llmprovider.NewAssistant),
	fx.Provide(NewAgents),
	fx.Provide(provideSessionService),
	fx.Provide(provideRunners),
	fx.Invoke(fx.Annotate(registerSessionCleaner, fx.ParamTags(``, `name:"ai_pool"`))),
)
