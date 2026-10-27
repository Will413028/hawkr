package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// Config 服務組態，全部來自環境變數，缺值時採預設。
type Config struct {
	RabbitMQ  RabbitMQConfig
	Database  DatabaseConfig
	Server    ServerConfig
	Env       string
	Inference InferenceConfig
	// MOABaseURL 農業部 OpenData API base（不含結尾斜線）。
	MOABaseURL         string
	MOAOpenDataBaseURL string
	// ProductServiceHost / ProductServicePort: category service（由
	// product-service 實作）的 gRPC 連線目標，issue #2786 的 admin mapping
	// 驗證用（internal/client/category）。
	ProductServiceHost string
	ProductServicePort string
	// MappingSeedCSVPath / MappingSeedManifestPath: P4 的商品↔作物種子表
	// （internal/domain/mapping）。預設值對齊 Dockerfile（Task 6）把 reference/
	// 複製進 image 的相對路徑，相對於 WORKDIR /app。
	MappingSeedCSVPath             string
	MappingSeedManifestPath        string
	PorkMarketRegistryCSVPath      string
	PorkMarketRegistryManifestPath string
	RiceCityRegistryCSVPath        string
	RiceCityRegistryManifestPath   string
	// GeographyMarketRegistryCSVPath / GeographyMarketRegistryManifestPath /
	// GeographyBoundaryGeoJSONPath / GeographyBoundaryManifestPath: the
	// category-aware market geography registry (internal/domain/geography,
	// PR #2321). Defaults mirror reference/'s actual filenames.
	GeographyMarketRegistryCSVPath      string
	GeographyMarketRegistryManifestPath string
	GeographyBoundaryGeoJSONPath        string
	GeographyBoundaryManifestPath       string
	// PriceScaleCSVPath / PriceScaleManifestPath: versioned fixed price
	// color scale for spatial-surface boundaryPriceGeojson/surfaceCellsGeojson
	// (internal/domain/spatial, issue #2248 PR B).
	PriceScaleCSVPath      string
	PriceScaleManifestPath string
	// ModelArtifactPath: 正式 model.json 的路徑，由 Docker image build 時
	// bake 進去（見 model/registry.json + cmd/pricebook-model-fetch）。預設值
	// 對齊 Dockerfile 把它 COPY 到的相對路徑，相對於 WORKDIR /app。
	ModelArtifactPath string
}

// InferenceConfig 是推論期的門檻與窗口。設計書 §8 line 423 要求「門檻放
// service config，不散落在元件」。
type InferenceConfig struct {
	// MinRidgeSampleCount：direct_ridge 的最小訓練樣本數，低於此值該點回
	// insufficient_data。0 停用整條閘門（forecast.EvaluatePointGates 對
	// <= 0 是停用而不是擋全部）。見 forecast.DefaultMinRidgeSampleCount 的依據。
	MinRidgeSampleCount int
	// FreshWithinHours / StaleWithinHours：設計書 §8 line 423 的 36h / 72h。
	FreshWithinHours int
	StaleWithinHours int
	// GroupVolumeWindowDays：大類加權的成交量回看天數（設計書 §8 是 28，
	// 上游 api/catalog.py 用 42）。
	GroupVolumeWindowDays int
	// ObservedWindowDays：對外要回的歷史觀測天數（設計書 §4 PriceSeries.observed
	// 註解寫「近 30 日」）。
	ObservedWindowDays int
	// GroupMinMarketSupportCount：issue #2249 Slice A dataMeta 大類聚合
	// usabilityStatus 的最低支援市場數門檻（forecast.GroupUsability 使用）。
	// 這是「正式門檻尚未確認」的暫代值，見
	// forecast.DefaultGroupMinMarketSupportCount 的完整依據與 ADR 引用。
	GroupMinMarketSupportCount int
}

// Validate 驗證推論門檻的形狀。MinRidgeSampleCount 允許 0（停用閘門，試營運
// 時想看更多預測用）；<0 沒有意義，一律報錯。新鮮度門檻交給
// forecast.FreshnessConfig.Validate()（FreshWithinHours >= StaleWithinHours
// 會讓 stale 區間消失）。
func (c InferenceConfig) Validate() error {
	if c.MinRidgeSampleCount < 0 {
		return fmt.Errorf("inference config: MIN_RIDGE_SAMPLE_COUNT must be >= 0, got %d", c.MinRidgeSampleCount)
	}
	if c.GroupVolumeWindowDays <= 0 {
		return fmt.Errorf("inference config: GROUP_VOLUME_WINDOW_DAYS must be positive, got %d", c.GroupVolumeWindowDays)
	}
	if c.ObservedWindowDays <= 0 {
		return fmt.Errorf("inference config: OBSERVED_WINDOW_DAYS must be positive, got %d", c.ObservedWindowDays)
	}
	if c.GroupMinMarketSupportCount < 0 {
		return fmt.Errorf("inference config: GROUP_MIN_MARKET_SUPPORT_COUNT must be >= 0, got %d", c.GroupMinMarketSupportCount)
	}
	freshness := forecast.FreshnessConfig{
		FreshWithin: time.Duration(c.FreshWithinHours) * time.Hour,
		StaleWithin: time.Duration(c.StaleWithinHours) * time.Hour,
	}
	if err := freshness.Validate(); err != nil {
		return fmt.Errorf("inference config: %w", err)
	}
	return nil
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RabbitMQConfig struct {
	URL string
}

// ServerConfig holds the final internal gRPC listener configuration. Operational
// health is served through grpc.health.v1 on the same listener; this service has
// no separate HTTP management port.
type ServerConfig struct {
	GRPCPort string
}

func (c *DatabaseConfig) DSN() string {
	return database.BuildDSNKeyword(database.DBConfig{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Name:     c.DBName,
		SSLMode:  c.SSLMode,
		TimeZone: "UTC",
	})
}

func Load() (*Config, error) {
	cfg := &Config{
		RabbitMQ: RabbitMQConfig{URL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "")},
		Env:      pkgconfig.GetEnvOrDefault("ENV", "development"),
		Database: DatabaseConfig{
			Host: pkgconfig.GetEnvOrDefault("DB_HOST", "localhost"),
			// ResolvePoolPort 優先讀 DB_POOL_PORT，prod 才會走到 PgBouncer；
			// 直接讀 DB_PORT 會讓本服務成為唯一對 PlanetScale 開直連的服務。
			Port:     database.ResolvePoolPort(),
			User:     pkgconfig.GetEnvOrDefault("DB_USER", "postgres"),
			Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   pkgconfig.GetEnvOrDefault("DB_NAME", "hawkr_pricebook"),
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10039"),
		},
		MOABaseURL:                          pkgconfig.GetEnvOrDefault("MOA_API_BASE_URL", "https://data.moa.gov.tw/api/v1"),
		MOAOpenDataBaseURL:                  pkgconfig.GetEnvOrDefault("MOA_OPENDATA_BASE_URL", "https://data.moa.gov.tw/Service/OpenData/FromM"),
		ProductServiceHost:                  pkgconfig.GetEnvOrDefault("PRODUCT_SERVICE_HOST", "127.0.0.1"),
		ProductServicePort:                  pkgconfig.GetEnvOrDefault("PRODUCT_SERVICE_GRPC_PORT", "9913"),
		MappingSeedCSVPath:                  pkgconfig.GetEnvOrDefault("MAPPING_SEED_CSV_PATH", "reference/product_crop_seed.csv"),
		PorkMarketRegistryCSVPath:           pkgconfig.GetEnvOrDefault("PORK_MARKET_REGISTRY_CSV_PATH", "reference/pork_market_registry.v1.csv"),
		PorkMarketRegistryManifestPath:      pkgconfig.GetEnvOrDefault("PORK_MARKET_REGISTRY_MANIFEST_PATH", "reference/pork_market_registry.v1.manifest.json"),
		RiceCityRegistryCSVPath:             pkgconfig.GetEnvOrDefault("RICE_CITY_REGISTRY_CSV_PATH", "reference/rice_city_registry.v1.csv"),
		RiceCityRegistryManifestPath:        pkgconfig.GetEnvOrDefault("RICE_CITY_REGISTRY_MANIFEST_PATH", "reference/rice_city_registry.v1.manifest.json"),
		MappingSeedManifestPath:             pkgconfig.GetEnvOrDefault("MAPPING_SEED_MANIFEST_PATH", "reference/product_crop_seed.manifest.json"),
		GeographyMarketRegistryCSVPath:      pkgconfig.GetEnvOrDefault("GEOGRAPHY_MARKET_REGISTRY_CSV_PATH", "reference/market_registry.v1.csv"),
		GeographyMarketRegistryManifestPath: pkgconfig.GetEnvOrDefault("GEOGRAPHY_MARKET_REGISTRY_MANIFEST_PATH", "reference/market_registry.v1.manifest.json"),
		GeographyBoundaryGeoJSONPath:        pkgconfig.GetEnvOrDefault("GEOGRAPHY_BOUNDARY_GEOJSON_PATH", "reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson"),
		GeographyBoundaryManifestPath:       pkgconfig.GetEnvOrDefault("GEOGRAPHY_BOUNDARY_MANIFEST_PATH", "reference/taiwan_boundary.v1.manifest.json"),
		PriceScaleCSVPath:                   pkgconfig.GetEnvOrDefault("PRICE_SCALE_CSV_PATH", "reference/price_scale.v1.csv"),
		PriceScaleManifestPath:              pkgconfig.GetEnvOrDefault("PRICE_SCALE_MANIFEST_PATH", "reference/price_scale.v1.manifest.json"),
		ModelArtifactPath:                   pkgconfig.GetEnvOrDefault("MODEL_ARTIFACT_PATH", "model/model.json"),
	}
	inference, err := loadInferenceConfig()
	if err != nil {
		return nil, err
	}
	cfg.Inference = inference
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadInferenceConfig 讀五個推論門檻環境變數。整數解析失敗必須報錯而不是靜默
// 用預設——一個打錯的環境變數靜默退回預設值，會讓「我明明調了門檻」變成無法
// 解釋的行為。
func loadInferenceConfig() (InferenceConfig, error) {
	var c InferenceConfig
	var err error
	if c.MinRidgeSampleCount, err = envInt("MIN_RIDGE_SAMPLE_COUNT", forecast.DefaultMinRidgeSampleCount); err != nil {
		return c, err
	}
	if c.FreshWithinHours, err = envInt("FRESH_WITHIN_HOURS", 36); err != nil {
		return c, err
	}
	if c.StaleWithinHours, err = envInt("STALE_WITHIN_HOURS", 72); err != nil {
		return c, err
	}
	if c.GroupVolumeWindowDays, err = envInt("GROUP_VOLUME_WINDOW_DAYS", forecast.DefaultGroupVolumeWindowDays); err != nil {
		return c, err
	}
	if c.ObservedWindowDays, err = envInt("OBSERVED_WINDOW_DAYS", 30); err != nil {
		return c, err
	}
	if c.GroupMinMarketSupportCount, err = envInt("GROUP_MIN_MARKET_SUPPORT_COUNT", forecast.DefaultGroupMinMarketSupportCount); err != nil {
		return c, err
	}
	return c, nil
}

// envInt 讀一個整數環境變數；未設定時採 fallback，設定但無法解析時報錯（不是
// 靜默退回 fallback）。
func envInt(key string, fallback int) (int, error) {
	raw := pkgconfig.GetEnvOrDefault(key, "")
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q: %w", key, raw, err)
	}
	return v, nil
}

// Validate 一次收集所有缺項再回報，避免部署時每修一個環境變數就重啟一輪才發現下一個。
func (c *Config) Validate() error {
	var missing []string
	if c.Database.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.Database.Port == "" {
		missing = append(missing, "DB_PORT")
	}
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if c.Database.DBName == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.Server.GRPCPort == "" {
		missing = append(missing, "GRPC_PORT")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if err := c.Inference.Validate(); err != nil {
		return err
	}
	if err := pkgotel.ValidateMetricsConfiguration(c.Env); err != nil {
		return err
	}
	return nil
}
