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
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

// Image 是單體統一的 Postgres 主版本：測試、Atlas 的 dev database、jet 的生成（cmd/jetgen）與本機 compose 都用它。
const Image = "postgres:18-alpine"

// templateFormat 是 template 建法本身的版本；改了建 template 的方式就加一，舊的 template 不再被用到。
const templateFormat = "1"

// testPoolConns 是每支測試的連線池上限。要容得下測試自己同時佔住的連線（例如一條持鎖加上四個並行請求各佔一條，
// 上限 4 時會互等到逾時），所以不能太小。
const testPoolConns = 10

// adminConnsPerTest 是每支測試同時最多佔用的管理連線：建立時一條、清理時一條，加上 template 的建構者一條，各自短暫。
const adminConnsPerTest = 3

// maxConnections 是 container 的 max_connections，容納整次 go test 的並行上界：
// go test 同時跑 -p（預設 NumCPU）個套件行程，每個行程同時跑 -parallel（預設 GOMAXPROCS，通常等於 NumCPU）支測試，
// 每支測試最多 testPoolConns + adminConnsPerTest 條連線，所以上界是 NumCPU² × 13，再留 50 條給 template 與管理。
// 8 核是 8×8×13+50 = 882。pgxpool 只在需要時才開連線，實際用量遠低於上界。
func maxConnections() string {
	n := runtime.NumCPU()
	return strconv.Itoa(max(100, n*n*(testPoolConns+adminConnsPerTest)+50))
}

// dbSeq 是行程內測試 database 的流水號；名稱 t_<pid>_<seq> 在整個 container 內唯一。
var dbSeq atomic.Int64

// Server 是一個測試用的 Postgres container。
type Server struct {
	container *tcpostgres.PostgresContainer
	shared    bool // 與同一次 go test 的其他套件行程共用；由 Ryuk 收掉，Close 不關
	adminURL  string

	mu        sync.Mutex
	templates map[string]*template
}

type template struct {
	once sync.Once
	err  error
}

// runFunc 是起 container 的函式（tcpostgres.Run）；測試可以換成替身。
type runFunc func(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*tcpostgres.PostgresContainer, error)

// Start 起（或找到同一次 go test 已經起好的）Postgres container。
func Start(ctx context.Context) (*Server, error) { return start(ctx, tcpostgres.Run) }

// start 在共用 container 時最多試兩次：找到的 container 可能在「找到」與「用到」之間被 Ryuk 回收
// （所有行程的連線都斷了超過 Ryuk 的 reconnection timeout），第二次會起一個新的。
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
	// 探測：找到的 container 要真的連得上，否則視為已被回收。
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
	for _, m := range mods {
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
		key:   "migrate " + hex.EncodeToString(h.Sum(nil)),
		apply: func(ctx context.Context, d *db.DB) error { return migrate.Apply(ctx, d, mods) },
	}
}

// Jobs 建立 River 的表（jobs.Migrate）。
func Jobs() Step {
	return Step{key: "river " + riverVersion(), apply: jobs.Migrate}
}

// riverVersion 是 River 的 module 版本：River 的 migration 跟著版本走。
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
func (s *Server) NewDB(t *testing.T, steps ...Step) *db.DB {
	t.Helper()
	d, err := db.Open(context.Background(), db.Config{URL: s.NewURL(t, steps...), MaxConns: testPoolConns, ConnectRetryBudget: 10 * time.Second})
	if err != nil {
		t.Fatalf("testdb：連到測試 database：%v", err)
	}
	t.Cleanup(d.Close)
	return d
}

// urlFor 是同一個 container 裡 database name 的連線字串。
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

// template 回傳 steps 的 template database，第一次用到時建立；同一個行程只建一次，
// 不同行程之間以 advisory lock 排隊，先到的建、後到的直接用。
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
	// 連線關閉就釋放 lock。

	var isTemplate bool
	switch err := admin.QueryRow(ctx, "SELECT datistemplate FROM pg_database WHERE datname = $1", name).Scan(&isTemplate); {
	case err == nil && isTemplate:
		return nil // 別的行程建好了
	case err == nil:
		// 別的行程建到一半就結束了：重建。
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
