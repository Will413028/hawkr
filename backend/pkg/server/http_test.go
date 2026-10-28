package server

import (
	"context"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

// TestNewRouter_ExposesOnlyOperationalRoutes pins this router's HTTP surface to
// operational endpoints only.
//
// Gateway is the single external HTTP entrypoint; inter-service traffic is gRPC.
// Every service built on this shared router exposes only operational routes: a
// business route on the internal HTTP port would sit behind nothing but gin.Recovery()
// — no authentication, no authorization — and duplicate RPCs the gateway already
// fronts. This test fails if any business route is wired back in.
func TestNewRouter_ExposesOnlyOperationalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := newRouter(ServerConfig{Env: "development"}, "content-service", zap.NewNop())

	want := map[string]bool{
		"GET /health":  true,
		"HEAD /health": true,
		"GET /metrics": true,
	}

	var unexpected []string
	for _, route := range r.Routes() {
		sig := route.Method + " " + route.Path
		if want[sig] {
			delete(want, sig)
			continue
		}
		if strings.HasPrefix(route.Path, "/debug/pprof") {
			continue
		}
		unexpected = append(unexpected, sig)
	}

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Errorf("router exposes non-operational routes: %v", unexpected)
	}
	for sig := range want {
		t.Errorf("expected operational route %q is missing", sig)
	}
}

// TestNewRouter_DropsPprofInProduction covers every spelling a production
// deployment can carry. The gate used to read os.Getenv("APP_ENV") while the
// deployed ConfigMaps only ever set ENV=prod, so on prod the variable was empty,
// `"" != "production"` held, and pprof — heap, goroutine, profile, trace — was
// live and unauthenticated on every service. The old test set APP_ENV=production
// itself, which is exactly what hid the mismatch. Drive the gate from the
// deployed value alone (issue #1418).
func TestNewRouter_DropsPprofInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, env := range []string{"prod", "production", "PROD", "Production", "  prod  "} {
		t.Run(env, func(t *testing.T) {
			r := newRouter(ServerConfig{Env: env}, "content-service", zap.NewNop())
			for _, route := range r.Routes() {
				if strings.HasPrefix(route.Path, "/debug/pprof") {
					t.Errorf("Env=%q: pprof route %s %s must not be registered in production", env, route.Method, route.Path)
				}
			}
		})
	}
}

func TestIsProductionEnv(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want bool
	}{
		{"prod", true},
		{"production", true},
		{"PROD", true},
		{"  production  ", true},
		{"dev", false},
		{"development", false},
		{"staging", false},
		{"", false},
	} {
		if got := IsProductionEnv(tc.env); got != tc.want {
			t.Errorf("IsProductionEnv(%q) = %v, want %v", tc.env, got, tc.want)
		}
	}
}

func TestNewRouter_ProductionGateUsesServerConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		serverEnv string
		appEnv    string
		wantPprof bool
	}{
		{name: "prod config disables pprof", serverEnv: "prod", appEnv: "development", wantPprof: false},
		{name: "production config disables pprof", serverEnv: "production", appEnv: "development", wantPprof: false},
		{name: "config wins over production APP_ENV", serverEnv: "development", appEnv: "production", wantPprof: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			r := newRouter(ServerConfig{Env: tc.serverEnv}, "review-service", zap.NewNop())
			gotPprof := false
			for _, route := range r.Routes() {
				if strings.HasPrefix(route.Path, "/debug/pprof") {
					gotPprof = true
					break
				}
			}
			if gotPprof != tc.wantPprof {
				t.Fatalf("pprof registered = %v, want %v", gotPprof, tc.wantPprof)
			}
		})
	}
}

func TestNewRouter_PprofBlockRegistersAllElevenPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := newRouter(ServerConfig{Env: "development"}, "content-service", zap.NewNop())

	wantPprof := []string{
		"GET /debug/pprof/",
		"GET /debug/pprof/cmdline",
		"GET /debug/pprof/profile",
		"GET /debug/pprof/symbol",
		"POST /debug/pprof/symbol",
		"GET /debug/pprof/trace",
		"GET /debug/pprof/allocs",
		"GET /debug/pprof/block",
		"GET /debug/pprof/goroutine",
		"GET /debug/pprof/heap",
		"GET /debug/pprof/mutex",
		"GET /debug/pprof/threadcreate",
	}

	got := map[string]bool{}
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	for _, sig := range wantPprof {
		if !got[sig] {
			t.Errorf("expected pprof route %q to be registered", sig)
		}
	}
}

func TestNewInternalHTTPServer_LifecycleHooksBindAndReleasePort(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	port := freePort(t)
	lc := fxtest.NewLifecycle(t)

	NewInternalHTTPServer(HTTPServerParams{
		Lifecycle:   lc,
		Config:      ServerConfig{ServerConfig: config.ServerConfig{HTTPPort: port}, Env: "production"},
		ServiceName: "social-service",
		Logger:      zap.NewNop(),
	})

	lc.RequireStart()

	client := &http.Client{
		Timeout: 200 * time.Millisecond,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		resp, err = client.Get("http://127.0.0.1:" + port + "/health")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("OnStart did not serve /health on :%s within the retry window: %v", port, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	lc.RequireStop()

	if _, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond); err == nil {
		t.Fatalf("OnStop (Shutdown) did not release :%s", port)
	}
}

func TestNewInternalHTTPServer_PropagatesBindError(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port)
	lc := fxtest.NewLifecycle(t)

	NewInternalHTTPServer(HTTPServerParams{
		Lifecycle: lc,
		Config: ServerConfig{
			ServerConfig: config.ServerConfig{HTTPPort: port},
			Env:          "production",
		},
		ServiceName: "review-service",
		Logger:      zap.NewNop(),
	})

	if err := lc.Start(context.Background()); err == nil {
		t.Fatal("HTTP lifecycle start returned nil for an occupied port")
	}
}
