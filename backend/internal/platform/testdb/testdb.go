// Package testdb 提供 integration 測試用的 Postgres。
//
// 一次 go test 只起一個 container，所有套件共用；每組 schema（模組 migration、River 的表）
// 只建一次，做成 template database；每支測試以 CREATE DATABASE … TEMPLATE 拿到自己的
// 一份（幾十毫秒），測試之間不會互相看到資料，可以 t.Parallel()。
//
// 用法：
//
//	var pg *testdb.Server
//	func TestMain(m *testing.M) { testdb.Main(m, &pg) }
//	func setup(t *testing.T) *db.DB {
//		return pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "seller", Dir: seller.Migrations()}))
//	}
//
// 不給 Step 時拿到空的 database（測 migration 本身的測試用）。
//
// template 的名稱是各 Step 內容（模組名、migration 目錄每個檔案的內容、River 的版本）的
// hash：migration 一改就是另一個 template，不會拿到舊的。template 設成不接受連線，只能拿來複製。
//
// 共用 container：container 名稱取自 testcontainers 的 session ID（go test 這個父行程的
// PID 與啟動時間），同一次 go test 的各套件行程找到同一個 container；最後一個行程結束後由
// Ryuk 刪除。Ryuk 被停用（TESTCONTAINERS_RYUK_DISABLED）時沒有人能在最後收掉共用的
// container，改成每個行程自己起、自己關。
//
// CI 的 integration 步驟把 TESTCONTAINERS_RYUK_RECONNECTION_TIMEOUT 設成 15m：-race 編譯大型
// package 時行程之間可能空窗超過 Ryuk 預設的 10 秒，Ryuk 會提早刪掉共用的 container。
package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/lockorder"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

// Image 是單體統一的 Postgres 主版本：測試、Atlas 的 dev database、jet 的生成（cmd/jetgen）與本機 compose 都用它。
const Image = "postgres:18-alpine"

const templateFormat = "2"

const sellerStoreIDSequenceStart int64 = 1_000_000_000_000

const testPoolConns = 10

const adminConnsPerTest = 3

func maxConnections() string {
	n := runtime.NumCPU()
	return strconv.Itoa(max(100, n*n*(testPoolConns+adminConnsPerTest)+50))
}

var dbSeq atomic.Int64

// Server 是一個測試用的 Postgres container。
type Server struct {
	container *tcpostgres.PostgresContainer
	shared    bool
	adminURL  string

	mu        sync.Mutex
	templates map[string]*template
}

type template struct {
	once sync.Once
	err  error
}

type runFunc func(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*tcpostgres.PostgresContainer, error)

// Start 起（或找到同一次 go test 已經起好的）Postgres container。
func Start(ctx context.Context) (*Server, error) { return start(ctx, tcpostgres.Run) }

func start(ctx context.Context, run runFunc) (*Server, error) {
	srv, err := startOnce(ctx, run)
	if err != nil && !testcontainers.ReadConfig().RyukDisabled {
		srv, err = startOnce(ctx, run)
	}
	return srv, err
}

func startOnce(ctx context.Context, run runFunc) (*Server, error) {
	shared := !testcontainers.ReadConfig().RyukDisabled
	opts := []testcontainers.ContainerCustomizer{
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithCmdArgs("-c", "max_connections="+maxConnections()),
		testcontainers.WithLabels(map[string]string{"hawkr.testdb": "1"}),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	}
	if shared {
		opts = append(opts, testcontainers.WithReuseByName("hawkr-testdb-"+testcontainers.SessionID()[:16]))
	}
	c, err := run(ctx, Image, opts...)
	if err != nil {
		return nil, fmt.Errorf("testdb：起 container：%w", err)
	}
	u, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		if !shared {
			_ = c.Terminate(context.Background())
		}
		return nil, fmt.Errorf("testdb：取得連線字串：%w", err)
	}
	probe, err := pgx.Connect(ctx, u)
	if err != nil {
		if !shared {
			_ = c.Terminate(context.Background())
		}
		return nil, fmt.Errorf("testdb：連到 container：%w", err)
	}
	_ = probe.Close(ctx)
	return &Server{container: c, shared: shared, adminURL: u, templates: map[string]*template{}}, nil
}

// Close 關閉這個行程自己的 container；共用的 container 留給 Ryuk 在最後一個行程結束後刪除。
func (s *Server) Close() {
	if !s.shared {
		_ = s.container.Terminate(context.Background())
	}
}

// Main 是整個 TestMain：起 container、跑測試、關 container。
// Docker 不可用時直接失敗，不略過：integration 測試沒跑卻顯示通過，比失敗更糟。
func Main(m *testing.M, s **Server) {
	srv, err := Start(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	*s = srv
	code := m.Run()
	srv.Close()
	os.Exit(code)
}

// Step 是測試 database 在測試開始前要有的一段 schema。內容相同的 Step key 相同。
type Step struct {
	key   string
	err   error
	apply func(context.Context, *db.DB) error
}

// Migrations 依序套用模組的 migration（migrate.Apply）。
func Migrations(mods ...migrate.Module) Step {
	h := sha256.New()
	hasSeller := false
	for _, m := range mods {
		if m.Name == "seller" {
			hasSeller = true
		}
		h.Write(fmt.Appendf(nil, "module %q\n", m.Name))
		err := fs.WalkDir(m.Dir, ".", func(p string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			b, err := fs.ReadFile(m.Dir, p)
			if err != nil {
				return err
			}
			h.Write(fmt.Appendf(nil, "file %q %d\n", p, len(b)))
			h.Write(b)
			return nil
		})
		if err != nil {
			return Step{err: fmt.Errorf("testdb：讀取 %s 的 migration：%w", m.Name, err)}
		}
	}
	return Step{
		key: "migrate " + hex.EncodeToString(h.Sum(nil)),
		apply: func(ctx context.Context, d *db.DB) error {
			if err := migrate.Apply(ctx, d, mods); err != nil {
				return err
			}
			if hasSeller {
				query := fmt.Sprintf("ALTER SEQUENCE seller.stores_store_id_seq RESTART WITH %d", sellerStoreIDSequenceStart)
				if _, err := d.Pool().Exec(ctx, query); err != nil {
					return fmt.Errorf("testdb：移動 seller.stores identity sequence：%w", err)
				}
			}
			return nil
		},
	}
}

// Jobs 建立 River 的表（jobs.Migrate）。
func Jobs() Step {
	return Step{key: "river " + riverVersion(), apply: jobs.Migrate}
}

func riverVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/riverqueue/river" {
				return dep.Version + " " + dep.Sum
			}
		}
	}
	return "unknown"
}

// NewURL 建一個這支測試專用、套用好 steps 的 database，回傳它的連線字串，並在測試結束時刪除。
func (s *Server) NewURL(t *testing.T, steps ...Step) string {
	t.Helper()
	ctx := context.Background()
	tpl, err := s.template(ctx, steps)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("t_%d_%d", os.Getpid(), dbSeq.Add(1))

	admin, err := pgx.Connect(ctx, s.adminURL)
	if err != nil {
		t.Fatalf("testdb：連到 admin database：%v", err)
	}
	defer func() {
		if err := admin.Close(ctx); err != nil {
			t.Errorf("testdb：關閉 admin database 連線：%v", err)
		}
	}()
	create := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize()
	if tpl != "" {
		create += " TEMPLATE " + pgx.Identifier{tpl}.Sanitize()
	}
	if _, err := admin.Exec(ctx, create); err != nil {
		t.Fatalf("testdb：建立 database：%v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, s.adminURL)
		if err != nil {
			t.Errorf("testdb：清理時連到 admin database：%v", err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Errorf("testdb：關閉清理用連線：%v", err)
			}
		}()
		if _, err := c.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("testdb：刪除 database：%v", err)
		}
	})
	u, err := s.urlFor(name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// NewDB 建一個這支測試專用、套用好 steps 的 database 並連上它，測試結束時關閉連線並刪除 database。
//
// 連線池掛著 lockorder.Checker：單體程式開的 transaction 第一次鎖某個模組的列時，已經持有排在它後面的模組的鎖
// （單體規則 4），或送出無法解析的 SQL，測試失敗並列出兩句 SQL 與呼叫處。測試檔自己開的 transaction 不檢查。
func (s *Server) NewDB(t *testing.T, steps ...Step) *db.DB {
	t.Helper()
	return s.newDB(t, t.Error, steps...)
}

func (s *Server) newDB(t *testing.T, report func(...any), steps ...Step) *db.DB {
	t.Helper()
	chk := lockorder.NewChecker()
	t.Cleanup(func() {
		for _, err := range chk.Errors() {
			report(err)
		}
		for _, v := range chk.Violations() {
			report(v)
		}
	})
	d, err := db.Open(context.Background(), db.Config{URL: s.NewURL(t, steps...), MaxConns: testPoolConns, ConnectRetryBudget: 10 * time.Second, Tracer: chk})
	if err != nil {
		t.Fatalf("testdb：連到測試 database：%v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func (s *Server) urlFor(name string) (string, error) {
	u, err := url.Parse(s.adminURL)
	if err != nil {
		return "", fmt.Errorf("testdb：解析連線字串：%w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

// TemplateName 是 steps 對應的 template database 名稱；沒有 step 時是空字串（空的 database）。
func TemplateName(steps ...Step) (string, error) {
	if len(steps) == 0 {
		return "", nil
	}
	keys := make([]string, len(steps))
	for i, st := range steps {
		if st.err != nil {
			return "", st.err
		}
		if st.apply == nil {
			return "", errors.New("testdb：Step 要用 Migrations 或 Jobs 建立")
		}
		keys[i] = st.key
	}
	sum := sha256.Sum256([]byte(templateFormat + "\n" + strings.Join(keys, "\n")))
	return "tpl_" + hex.EncodeToString(sum[:12]), nil
}

func (s *Server) template(ctx context.Context, steps []Step) (string, error) {
	name, err := TemplateName(steps...)
	if err != nil || name == "" {
		return name, err
	}
	s.mu.Lock()
	tp, ok := s.templates[name]
	if !ok {
		tp = &template{}
		s.templates[name] = tp
	}
	s.mu.Unlock()
	tp.once.Do(func() { tp.err = s.buildTemplate(ctx, name, steps) })
	return name, tp.err
}

func (s *Server) buildTemplate(ctx context.Context, name string, steps []Step) (err error) {
	admin, err := pgx.Connect(ctx, s.adminURL)
	if err != nil {
		return fmt.Errorf("testdb：連到 admin database：%w", err)
	}
	defer func() { err = errors.Join(err, admin.Close(context.WithoutCancel(ctx))) }()

	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", name); err != nil {
		return fmt.Errorf("testdb：取得 template 的 advisory lock：%w", err)
	}

	var isTemplate bool
	switch err := admin.QueryRow(ctx, "SELECT datistemplate FROM pg_database WHERE datname = $1", name).Scan(&isTemplate); {
	case err == nil && isTemplate:
		return nil
	case err == nil:
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			return fmt.Errorf("testdb：刪除沒建完的 template：%w", err)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("testdb：查詢 template：%w", err)
	}

	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		return fmt.Errorf("testdb：建立 template：%w", err)
	}
	u, err := s.urlFor(name)
	if err != nil {
		return err
	}
	d, err := db.Open(ctx, db.Config{URL: u, ConnectRetryBudget: 10 * time.Second})
	if err != nil {
		return fmt.Errorf("testdb：連到 template：%w", err)
	}
	for _, st := range steps {
		if err := st.apply(ctx, d); err != nil {
			d.Close()
			return fmt.Errorf("testdb：建立 template（%s）：%w", st.key, err)
		}
	}
	d.Close()
	if _, err := admin.Exec(ctx, "ALTER DATABASE "+ident+" WITH IS_TEMPLATE true ALLOW_CONNECTIONS false"); err != nil {
		return fmt.Errorf("testdb：標記 template：%w", err)
	}
	return nil
}

// StressEnv 是開啟壓力測試的環境變數；值為 1 時 RequireStress 才讓測試執行。
const StressEnv = "HAWKR_STRESS_TESTS"

// RequireStress 讓重複上百輪、檢查並行交錯（死鎖、不變式）的壓力測試只在 StressEnv=1 時執行，
// 否則略過。CI 只在要合併進 main（上線前）時開啟。
func RequireStress(t *testing.T) {
	t.Helper()
	if os.Getenv(StressEnv) != "1" {
		t.Skipf("壓力測試；設 %s=1 才執行", StressEnv)
	}
}
