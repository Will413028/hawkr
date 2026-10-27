// Package dbtest provides the postgres harness that the Go services' integration
// tests share: one container per test package, and one database of its own —
// cloned from a migrated template — per test.
//
// Why one container per package rather than one per test: creating a container
// copies the whole db/migrations/ directory into it, and the self-hosted
// runner's Docker storage driver intermittently fails that copy with "RWLayer
// of container ... is unexpectedly nil" (#2070) — a red build that is not a
// test failure, and the kind of red that teaches people to rerun instead of
// read. Booting once per package instead of once per test is fewer rolls of
// that dice.
//
// Why a database per test rather than a shared one: isolation is not what pays
// for the change, but it is load-bearing in every package that uses this.
// CREATE DATABASE ... TEMPLATE gives each test a pristine copy of the migrated
// schema, so a test still owns an entire database, exactly as it did when it
// owned an entire container. That matters for the assertions that are about a
// whole table rather than about one row — unbounded reads, counts, "no row
// matches this id" — which would silently become order-dependent the moment
// tests shared a database. Cross-test pollution is a worse outcome than a flaky
// container: it produces wrong answers instead of obvious failures.
//
// What this package deliberately does NOT own: how a test turns its database
// into a connection. Callers do that themselves in a handful of lines, because
// it genuinely differs — pgxpool.New, pgxpool with QueryExecModeExec,
// database.NewPool, gorm.Open — and folding those into one signature would
// produce a helper that serves none of them well.
package dbtest

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// 🔴 image 必須是 postgres:16-alpine：.github/workflows/mirror-testcontainers-images.yml:26
	// 只把這一個 image 鏡像到 GHCR，self-hosted runner 連不到 Docker Hub，其他 tag 在 CI 會失敗。
	image = "postgres:16-alpine"

	// adminDB is the maintenance database used to issue CREATE/DROP DATABASE,
	// which cannot be run from inside the database being created or dropped.
	adminDB = "postgres"

	pgUser     = "test"
	pgPassword = "test"

	startupTimeout = 90 * time.Second
)

// Config describes the template database a Postgres hands out copies of.
type Config struct {
	// TemplateDB is the database the migrations run in (via the container's
	// init-script hook). It is only ever read as a CREATE DATABASE template:
	// nothing connects to it, because a template with live connections cannot
	// be cloned.
	TemplateDB string

	// MigrationsDir holds the .sql files to replay, in filename order. A
	// relative path is resolved against the test's working directory, which is
	// the directory of the package under test (e.g. "../../db/migrations").
	MigrationsDir string
}

// Postgres lazily boots a container per distinct migration set and clones a
// database out of it for each test. The zero value is not usable; call New.
type Postgres struct {
	cfg Config

	mu        sync.Mutex
	instances map[string]*instance

	seq atomic.Int64
}

// instance is one container carrying one migrated template database. There is
// one per distinct excludeMigrations set rather than one per Postgres, because
// the excluded migrations are baked into the template by the init-script
// replay: a transitional schema and the final schema cannot both live in the
// same template. Tests that exclude nothing — the overwhelming majority — all
// share the same instance.
type instance struct {
	container *tcpostgres.PostgresContainer
	host      string
	port      string
	err       error // set once startup has been tried and lost
}

// New returns a Postgres that hands out copies of cfg.TemplateDB. Nothing is
// started until the first Clone.
func New(cfg Config) *Postgres {
	return &Postgres{cfg: cfg, instances: map[string]*instance{}}
}

// DB is one test's own database: the fully migrated schema, no rows.
type DB struct {
	// DSN connects to this database as the test user.
	DSN string

	// Host, Port, Name, User and Password are the same connection, taken
	// apart, for callers that configure a subject under test through
	// environment variables rather than a DSN.
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

// Clone returns a database of this test's own plus a func that drops it.
//
// The returned func is safe to call more than once, and safe to call in the
// middle of a test: dropping the database is how a test makes its database
// dead, the same way terminating the container used to be.
//
// excludeMigrations names migration files (basenames) to leave out when
// building the template. It is for tests that exercise logic written for a
// transitional schema state — e.g. a nullable column a later migration in the
// same directory makes NOT NULL — since a fully migrated schema can never hold
// the value the transitional code is testing for. Each distinct set costs one
// more container, so the common case passes none.
func (p *Postgres) Clone(ctx context.Context, t *testing.T, excludeMigrations ...string) (DB, func()) {
	t.Helper()

	// Sorted so that two call sites naming the same migrations in a different
	// order share one instance instead of booting two identical containers.
	exclude := append([]string(nil), excludeMigrations...)
	sort.Strings(exclude)

	inst := p.start(ctx, t, exclude)
	name := fmt.Sprintf("test_%d", p.seq.Add(1))

	admin, err := pgx.Connect(ctx, p.dsn(inst, adminDB))
	if err != nil {
		t.Fatalf("connect to %s to clone the template database: %v", adminDB, err)
	}
	_, execErr := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{name}.Sanitize(), pgx.Identifier{p.cfg.TemplateDB}.Sanitize()))
	closeErr := admin.Close(ctx)
	if execErr != nil {
		t.Fatalf("clone the migrated template database: %v", execErr)
	}
	if closeErr != nil {
		t.Fatalf("close the admin connection: %v", closeErr)
	}

	db := DB{
		DSN:      p.dsn(inst, name),
		Host:     inst.host,
		Port:     inst.port,
		Name:     name,
		User:     pgUser,
		Password: pgPassword,
	}

	drop := func() {
		// Dropped rather than left behind so a long run does not accumulate one
		// copy of the schema per test. FORCE covers connections opened outside
		// whatever pool the caller built; a failure here is not the test's
		// verdict, so it is logged rather than fatal.
		dropCtx := context.Background()
		admin, err := pgx.Connect(dropCtx, p.dsn(inst, adminDB))
		if err != nil {
			t.Logf("drop %s: connect: %v", name, err)
			return
		}
		defer func() { _ = admin.Close(dropCtx) }()
		if _, err := admin.Exec(dropCtx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)",
			pgx.Identifier{name}.Sanitize())); err != nil {
			t.Logf("drop %s: %v", name, err)
		}
	}
	return db, drop
}

// Close terminates every container this Postgres started. Call it from
// TestMain, or use RunTests.
func (p *Postgres) Close() {
	p.mu.Lock()
	live := make([]*tcpostgres.PostgresContainer, 0, len(p.instances))
	for _, inst := range p.instances {
		if inst.container != nil {
			live = append(live, inst.container)
		}
	}
	p.mu.Unlock()

	for _, c := range live {
		_ = c.Terminate(context.Background())
	}
}

// RunTests is the whole body of a package's TestMain: run the tests, then
// terminate the containers p started.
//
//	func TestMain(m *testing.M) { dbtest.RunTests(m, pg) }
func RunTests(m *testing.M, p *Postgres) {
	code := m.Run()
	p.Close()
	os.Exit(code)
}

// start boots the container for this migration set on first use. Lazy rather
// than eager in TestMain so that a run filtered with -run, or one that touches
// only the package's DB-free tests, pays only for the instances its tests
// actually ask for.
func (p *Postgres) start(ctx context.Context, t *testing.T, exclude []string) *instance {
	t.Helper()

	key := strings.Join(exclude, "\x00")

	p.mu.Lock()
	defer p.mu.Unlock()
	if inst, ok := p.instances[key]; ok {
		// A startup that already failed is not retried per test: the run is red
		// either way, and the point of this package is to stop hammering a
		// Docker daemon that is having a bad day.
		if inst.err != nil {
			t.Fatalf("shared postgres container is unavailable: %v", inst.err)
		}
		return inst
	}

	scripts := p.scripts(t, exclude)

	// One container now carries many tests, so a startup that lost the #2070
	// coin toss takes down all of them rather than one. Retry once: the failure
	// being covered is the daemon's, and a second attempt is cheaper than a
	// rerun of the job. Any startup error is retried, not just the RWLayer one —
	// matching a daemon error string is brittle, and a genuinely broken
	// environment fails the second time too.
	var container *tcpostgres.PostgresContainer
	var startErr error
	for attempt := 1; attempt <= 2; attempt++ {
		container, startErr = runPostgres(ctx, p.cfg.TemplateDB, scripts)
		if startErr == nil {
			break
		}
		t.Logf("postgres container start attempt %d/2 failed: %v", attempt, startErr)
		if container != nil {
			_ = container.Terminate(ctx)
			container = nil
		}
	}

	inst := &instance{container: container, err: startErr}
	p.instances[key] = inst
	if startErr != nil {
		t.Fatalf("start shared postgres container: %v", startErr)
	}

	host, err := container.Host(ctx)
	if err != nil {
		inst.err = err
		t.Fatalf("read the container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		inst.err = err
		t.Fatalf("read the container port: %v", err)
	}

	inst.host = host
	inst.port = port.Port()
	return inst
}

// scripts lists MigrationsDir in apply order, minus the excluded basenames.
func (p *Postgres) scripts(t *testing.T, exclude []string) []string {
	t.Helper()

	skip := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		skip[name] = true
	}

	migDir, err := filepath.Abs(p.cfg.MigrationsDir)
	if err != nil {
		t.Fatalf("resolve %s: %v", p.cfg.MigrationsDir, err)
	}
	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("read %s: %v", migDir, err)
	}
	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" && !skip[e.Name()] {
			scripts = append(scripts, filepath.Join(migDir, e.Name()))
		}
	}
	sort.Strings(scripts)
	if len(scripts) == 0 {
		t.Fatalf("no migration scripts found in %s", migDir)
	}
	return scripts
}

func runPostgres(ctx context.Context, templateDB string, scripts []string) (*tcpostgres.PostgresContainer, error) {
	return tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase(templateDB),
		tcpostgres.WithUsername(pgUser),
		tcpostgres.WithPassword(pgPassword),
		tcpostgres.WithInitScripts(scripts...),
		testcontainers.WithWaitStrategy(
			// postgres started with init scripts boots twice: the first "ready" is
			// the init phase, the second is the real server. Waiting for the port
			// alone races that restart and yields "connection reset by peer", so we
			// also wait for the second readiness log line.
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(startupTimeout),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
}

func (p *Postgres) dsn(inst *instance, database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		pgUser, pgPassword, net.JoinHostPort(inst.host, inst.port), database)
}
