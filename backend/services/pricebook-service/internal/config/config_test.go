package config

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// TestLoadUsesDefaults 驗證缺少環境變數時採用預設值。
// 用 t.Setenv(key, "") 而非 os.Clearenv()：GetEnvOrDefault 視空字串為未設定，
// 因此能確定性地觸發 fallback，且測試結束後自動還原（不污染同一 test binary 的其他測試）。
func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("GRPC_PORT", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("MOA_API_BASE_URL", "")
	t.Setenv("PRODUCT_SERVICE_HOST", "")
	t.Setenv("PRODUCT_SERVICE_GRPC_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 回傳非預期錯誤: %v", err)
	}
	if cfg.Server.GRPCPort != "10039" {
		t.Errorf("GRPCPort = %q，預期 10039", cfg.Server.GRPCPort)
	}
	if cfg.RabbitMQ.URL != "" {
		t.Error("local default must allow an unconfigured RabbitMQ")
	}
	if cfg.MOABaseURL != "https://data.moa.gov.tw/api/v1" {
		t.Errorf("MOABaseURL = %q", cfg.MOABaseURL)
	}
	if cfg.Database.DBName != "hawkr_pricebook" {
		t.Errorf("DBName = %q", cfg.Database.DBName)
	}
	// ProductServiceHost / ProductServicePort：category service（product-service
	// 實作）的 gRPC 連線目標，issue #2786 admin mapping 驗證用。預設值對齊
	// ai-service 既有同用途欄位（同一個 category service）。
	if cfg.ProductServiceHost != "127.0.0.1" {
		t.Errorf("ProductServiceHost = %q，預期 127.0.0.1", cfg.ProductServiceHost)
	}
	if cfg.ProductServicePort != "9913" {
		t.Errorf("ProductServicePort = %q，預期 9913", cfg.ProductServicePort)
	}
}

// TestLoadReadsEnv 驗證環境變數會覆蓋預設值。
func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://localhost:5672/pricebook-test")
	t.Setenv("GRPC_PORT", "9998")
	t.Setenv("DB_NAME", "custom")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 回傳非預期錯誤: %v", err)
	}
	if cfg.Server.GRPCPort != "9998" || cfg.Database.DBName != "custom" {
		t.Errorf("環境變數未生效: GRPCPort=%q DBName=%q", cfg.Server.GRPCPort, cfg.Database.DBName)
	}
	if cfg.RabbitMQ.URL != "amqp://localhost:5672/pricebook-test" {
		t.Error("RABBITMQ_URL was not loaded")
	}
}

// TestLoadPrefersPoolPortOverDirectPort 驗證 Load() 透過
// database.ResolvePoolPort() 解出的 port 優先取 DB_POOL_PORT，
// 兩者同時設定時不能落回 DB_PORT——本服務故意不直連 PlanetScale（見
// config.go 的 Load() 註解），這個優先序一旦回歸會讓 pricebook 對 prod
// 開出唯一一條繞過 PgBouncer 的直連（final whole-branch review M5）。
func TestLoadPrefersPoolPortOverDirectPort(t *testing.T) {
	t.Setenv("DB_POOL_PORT", "6432")
	t.Setenv("DB_PORT", "5432")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 回傳非預期錯誤: %v", err)
	}
	if cfg.Database.Port != "6432" {
		t.Errorf("Database.Port = %q，預期優先採用 DB_POOL_PORT=6432", cfg.Database.Port)
	}
}

// TestDatabaseConfig_DSN 驗證 DSN 帶齊 pgx 連線所需欄位。
func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "admin",
		Password: "secret",
		DBName:   "hawkr_pricebook",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	for _, s := range []string{"localhost", "5432", "admin", "secret", "hawkr_pricebook", "sslmode=disable"} {
		if !strings.Contains(dsn, s) {
			t.Errorf("DSN 缺少 %q: %s", s, dsn)
		}
	}
}

// TestConfig_Validate 驗證缺值會被一次回報完整清單，而不是只回報第一個。
func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "db-host",
				Port:     "5432",
				User:     "user",
				Password: "pass",
				DBName:   "hawkr_pricebook",
			},
			Server:    ServerConfig{GRPCPort: "10039"},
			Inference: defaultInferenceConfigForTest(),
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("預期 nil，得到 %v", err)
		}
	})

	t.Run("multiple missing fields reported together", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("預期回傳錯誤")
		}
		for _, field := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "GRPC_PORT"} {
			if !strings.Contains(err.Error(), field) {
				t.Errorf("錯誤訊息應提及 %s: %v", field, err)
			}
		}
	})
}

func TestConfigValidate_ProductionRequiresOTLPMetricsEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := validConfigForTest()
	cfg.Env = "production"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") {
		t.Fatalf("production config error = %v, want missing OTLP metrics endpoint", err)
	}
}

func TestConfigValidate_DevelopmentAllowsMissingOTLPMetricsEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := validConfigForTest()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development config should allow missing OTLP metrics endpoint: %v", err)
	}
}

func validConfigForTest() *Config {
	return &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "db-host",
			Port:     "5432",
			User:     "user",
			Password: "pass",
			DBName:   "hawkr_pricebook",
		},
		Server:    ServerConfig{GRPCPort: "10039"},
		Inference: defaultInferenceConfigForTest(),
	}
}

// defaultInferenceConfigForTest 是 loadInferenceConfig() 五個預設值的字面複製，
// 供不透過 Load() 直接建構 Config 的測試使用（TestConfig_Validate 的
// "valid config" case）。刻意不呼叫 loadInferenceConfig() 本身：那樣會讓這個
// helper 依賴環境變數狀態，測試就不再是純值比對。
func defaultInferenceConfigForTest() InferenceConfig {
	return InferenceConfig{
		MinRidgeSampleCount:        forecast.DefaultMinRidgeSampleCount,
		FreshWithinHours:           36,
		StaleWithinHours:           72,
		GroupVolumeWindowDays:      forecast.DefaultGroupVolumeWindowDays,
		ObservedWindowDays:         30,
		GroupMinMarketSupportCount: forecast.DefaultGroupMinMarketSupportCount,
	}
}

// TestLoadInferenceDefaults 驗證五個推論門檻環境變數缺省時採用預設值
// （config_test.go 慣例：t.Setenv(key, "") 而非 os.Clearenv()，理由同
// TestLoadUsesDefaults 的檔頭註解）。
func TestLoadInferenceDefaults(t *testing.T) {
	for _, key := range []string{
		"MIN_RIDGE_SAMPLE_COUNT", "FRESH_WITHIN_HOURS", "STALE_WITHIN_HOURS",
		"GROUP_VOLUME_WINDOW_DAYS", "OBSERVED_WINDOW_DAYS", "GROUP_MIN_MARKET_SUPPORT_COUNT",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 回傳非預期錯誤: %v", err)
	}
	want := defaultInferenceConfigForTest()
	if cfg.Inference != want {
		t.Errorf("Inference = %+v，預期預設值 %+v", cfg.Inference, want)
	}
}

// TestLoadInferenceEnvOverride 驗證環境變數覆寫五個推論門檻生效。
func TestLoadInferenceEnvOverride(t *testing.T) {
	t.Setenv("MIN_RIDGE_SAMPLE_COUNT", "0")
	t.Setenv("FRESH_WITHIN_HOURS", "12")
	t.Setenv("STALE_WITHIN_HOURS", "48")
	t.Setenv("GROUP_VOLUME_WINDOW_DAYS", "14")
	t.Setenv("OBSERVED_WINDOW_DAYS", "60")
	t.Setenv("GROUP_MIN_MARKET_SUPPORT_COUNT", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 回傳非預期錯誤: %v", err)
	}
	want := InferenceConfig{
		MinRidgeSampleCount: 0, FreshWithinHours: 12, StaleWithinHours: 48,
		GroupVolumeWindowDays: 14, ObservedWindowDays: 60, GroupMinMarketSupportCount: 3,
	}
	if cfg.Inference != want {
		t.Errorf("Inference = %+v，預期 %+v（含 MIN_RIDGE_SAMPLE_COUNT=0 必須被接受，"+
			"這是試營運想停用整條 ridge 樣本量閘門的唯一開關）", cfg.Inference, want)
	}
}

// TestLoadInferenceRejectsNonInteger 驗證打錯的環境變數會報錯，而不是靜默退回
// 預設值——一個打錯的環境變數靜默退回預設值，會讓「我明明調了門檻」變成無法
// 解釋的行為（task-11-brief.md Step 1）。五個 key 逐一測，確認 envInt 對每一個
// 呼叫點都生效，不是只測到第一個就以為其餘四個也對。
func TestLoadInferenceRejectsNonInteger(t *testing.T) {
	for _, key := range []string{
		"MIN_RIDGE_SAMPLE_COUNT", "FRESH_WITHIN_HOURS", "STALE_WITHIN_HOURS",
		"GROUP_VOLUME_WINDOW_DAYS", "OBSERVED_WINDOW_DAYS", "GROUP_MIN_MARKET_SUPPORT_COUNT",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "not-an-integer")
			_, err := Load()
			if err == nil {
				t.Fatalf("%s=\"not-an-integer\" 應報錯，不能靜默退回預設值", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("錯誤訊息應提及 %s，方便操作者定位是哪個變數打錯: %v", key, err)
			}
		})
	}
}

// TestInferenceConfigValidate_MinRidgeSampleCountZeroDisablesGate 釘住
// MIN_RIDGE_SAMPLE_COUNT=0 不會被 config 層擋掉——0 是「停用整條閘門」的合法值
// （forecast.EvaluatePointGates 對 <= 0 是停用而不是擋全部），不是一個該被
// Validate() 拒絕的「無效」輸入。負值才是真的無效。
func TestInferenceConfigValidate_MinRidgeSampleCountZeroDisablesGate(t *testing.T) {
	cfg := defaultInferenceConfigForTest()
	cfg.MinRidgeSampleCount = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("MinRidgeSampleCount=0 應被接受，得到錯誤: %v", err)
	}

	cfg.MinRidgeSampleCount = -1
	if err := cfg.Validate(); err == nil {
		t.Error("MinRidgeSampleCount=-1 應報錯（負數沒有意義）")
	}
}
