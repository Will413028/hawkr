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
type Checker struct {
	mu         sync.Mutex
	txs        map[any]*txState
	violations []Violation
	seen       map[string]bool
	errs       []error
}

// txState 是一個 transaction 到目前為止的加鎖：排最後的那一次，以及鎖過哪些模組。
//
// 判定的單位是「每個模組第一次加鎖」：一個模組第一次被鎖時，不能已經鎖過排在它後面的模組。同一個模組之後再鎖
// 不算倒退，因為規則 4 的做法是先鎖下層的根列（例如 identity 帳號列），同一個帳號的其他 transaction 都排在它
// 後面，之後回頭寫同一個模組的其他列不會形成等待的環。這也表示：在同一個模組裡鎖到「另一個根」的列（例如另一個
// 帳號）不會被抓到，那要靠模組內的順序與個案的死鎖測試。
//
// 測試檔自己開的 transaction（fixture 直接呼叫只該在別人的 transaction 裡用的方法、並行測試故意持有鎖來模擬交錯）
// 不檢查：那是測試自己組出來的順序，不是單體的 use case。判斷方式是開 transaction 的呼叫者在 _test.go 裡；
// platform 自己的測試不算（呼叫處會略過 platform 的 frame），所以這個套件與 testdb 的測試照常檢查。
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

// observe 處理 key（一條連線）上的一句 SQL。
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
		if st.held && rank < st.topRank && l.Waits && !st.locked[rank] && !exempt(st.top, acq) {
			c.record(Violation{Held: st.top, Then: acq})
		}
		st.locked[rank] = true
		if !st.held || rank > st.topRank {
			st.top, st.topRank, st.held = acq, rank, true
		}
		c.mu.Unlock()
	}
}

// record 在持有 c.mu 時呼叫。
func (c *Checker) record(v Violation) {
	k := v.Held.Statement + "\x00" + v.Held.Site + "\x00" + v.Then.Statement + "\x00" + v.Then.Site
	if c.seen[k] {
		return
	}
	c.seen[k] = true
	c.violations = append(c.violations, v)
}

// txBoundary 認出 pgx 送出的 transaction 邊界語句。
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

// statementName 回傳 sqlc 寫在 query 第一行的名稱（-- name: Foo :one）；其他的 SQL 取壓縮空白後的前 80 個字。
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

// skipFrames 是不算「呼叫處」的套件：交易與 tracer 本身、pgx、生成的 store 與 jet 程式。
var skipFrames = []string{
	modulePath + "internal/platform/",
	"github.com/jackc/",
	"/internal/store.",
	"/jet/",
}

// callSite 回傳單體程式裡最近的呼叫者；找不到時回傳空字串。
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
