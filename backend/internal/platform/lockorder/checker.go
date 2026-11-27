package lockorder

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
)

// Acquisition 是 transaction 裡的一次加鎖：哪一句 SQL、鎖了什麼、從哪裡呼叫。
type Acquisition struct {
	Lock      Lock
	Statement string // sqlc 的 query 名稱；不是 sqlc 的 query 時是壓縮空白後的前 80 個字
	Site      string // 單體程式裡最近的呼叫者（略過 platform 與生成的 store），格式 path:line
}

// Violation 是同一個 transaction 第一次鎖某個模組的列時，已經持有排在它後面的模組的鎖。
type Violation struct {
	Held Acquisition
	Then Acquisition
}

func (v Violation) String() string {
	return fmt.Sprintf("鎖順序倒退：先鎖 %s（%s，%s，%s），再等 %s（%s，%s，%s）；全域順序見 platform/lockorder.Modules（backend/AGENTS.md 規則 4）",
		qualified(v.Held.Lock), v.Held.Lock.Kind, v.Held.Statement, v.Held.Site,
		qualified(v.Then.Lock), v.Then.Lock.Kind, v.Then.Statement, v.Then.Site)
}

func qualified(l Lock) string { return l.Schema + "." + l.Table }

// Checker 是記錄每個 transaction 加鎖順序的 pgx.QueryTracer。一個 Checker 給一個連線池用；只在測試裡掛。
//
// transaction 的邊界看 pgx 送出的 begin、commit、rollback（db.InTx 與 InReadSnapshot 都經 pgx 的 BeginTx）；
// 不在 transaction 裡的單句 SQL 沒有順序可言，不檢查。
//
// COPY FROM 不追蹤（沒有實作 pgx.CopyFromTracer）：它只新增列，表上是 RowExclusiveLock，不鎖既有的列，也沒有
// ON CONFLICT，依本套件的規則和單純的 INSERT 一樣不算列鎖。
type Checker struct {
	mu         sync.Mutex
	txs        map[any]*txState
	violations []Violation
	seen       map[string]bool
	errs       []error
}

type txState struct {
	top     Acquisition
	topRank int
	held    bool
	locked  map[int]bool
	byTest  bool
}

// NewChecker 建一個空的 Checker。
func NewChecker() *Checker {
	return &Checker{txs: map[any]*txState{}, seen: map[string]bool{}}
}

// TraceQueryStart 實作 pgx.QueryTracer。
func (c *Checker) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	c.observe(conn, data.SQL)
	return ctx
}

// TraceQueryEnd 實作 pgx.QueryTracer；鎖在語句開始時就算數（等待發生在執行中），結束時不用做事。
func (c *Checker) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// TraceBatchStart 實作 pgx.BatchTracer：SendBatch 不經 TraceQueryStart，這裡把 batch 裡排好的每一句 SQL 依序
// 當成單句處理（batch 在同一條連線上按順序執行，鎖在語句執行時加）。
func (c *Checker) TraceBatchStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchStartData) context.Context {
	if data.Batch == nil {
		return ctx
	}
	for _, q := range data.Batch.QueuedQueries {
		c.observe(conn, q.SQL)
	}
	return ctx
}

// TraceBatchQuery 實作 pgx.BatchTracer；語句已在 TraceBatchStart 處理過，這裡不重複算。
func (c *Checker) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {}

// TraceBatchEnd 實作 pgx.BatchTracer。
func (c *Checker) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

// Violations 回傳到目前為止的倒退，同一組（先鎖的語句與呼叫處、後鎖的語句與呼叫處）只記一次。
func (c *Checker) Violations() []Violation {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Violation(nil), c.violations...)
}

// Errors 回傳無法解析的 SQL：解析不了就無從檢查，測試應該看到它，而不是默默跳過。
func (c *Checker) Errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]error(nil), c.errs...)
}

func (c *Checker) observe(key any, sql string) {
	switch txBoundary(sql) {
	case "begin":
		byTest := strings.Contains(callSite(), "_test.go:")
		c.mu.Lock()
		c.txs[key] = &txState{locked: map[int]bool{}, byTest: byTest}
		c.mu.Unlock()
		return
	case "end":
		c.mu.Lock()
		delete(c.txs, key)
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	st := c.txs[key]
	c.mu.Unlock()
	if st == nil || st.byTest {
		return
	}
	locks, err := Classify(sql)
	if err != nil {
		c.mu.Lock()
		c.errs = append(c.errs, err)
		c.mu.Unlock()
		return
	}
	var site string
	for _, l := range locks {
		rank, ok := Rank(l.Schema)
		if !ok {
			continue
		}
		if site == "" {
			site = callSite()
		}
		acq := Acquisition{Lock: l, Statement: statementName(sql), Site: site}
		c.mu.Lock()
		if st.held && rank < st.topRank && l.Waits && !st.locked[rank] {
			c.record(Violation{Held: st.top, Then: acq})
		}
		if l.Waits {
			st.locked[rank] = true
		}
		if !st.held || rank > st.topRank {
			st.top, st.topRank, st.held = acq, rank, true
		}
		c.mu.Unlock()
	}
}

func (c *Checker) record(v Violation) {
	k := v.Held.Statement + "\x00" + v.Held.Site + "\x00" + v.Then.Statement + "\x00" + v.Then.Site
	if c.seen[k] {
		return
	}
	c.seen[k] = true
	c.violations = append(c.violations, v)
}

func txBoundary(sql string) string {
	s := strings.ToLower(strings.TrimSpace(sql))
	switch {
	case s == "begin" || strings.HasPrefix(s, "begin "):
		return "begin"
	case s == "commit" || s == "rollback":
		return "end"
	}
	return ""
}

func statementName(sql string) string {
	s := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(s, "-- name: "); ok {
		if name, _, ok := strings.Cut(rest, " "); ok {
			return name
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}

const modulePath = "github.com/Will413028/hawkr/backend/"

var skipFrames = []string{
	modulePath + "internal/platform/",
	"github.com/jackc/",
	"/internal/store.",
	"/jet/",
}

func callSite() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if strings.HasPrefix(f.Function, modulePath) && !skipped(f.Function) {
			file := f.File
			if i := strings.Index(file, "/backend/"); i >= 0 {
				file = file[i+len("/backend/"):]
			}
			return fmt.Sprintf("%s:%d", file, f.Line)
		}
		if !more {
			return ""
		}
	}
}

func skipped(fn string) bool {
	for _, s := range skipFrames {
		if strings.Contains(fn, s) {
			return true
		}
	}
	return false
}
