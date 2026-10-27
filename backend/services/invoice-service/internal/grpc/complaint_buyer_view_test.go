package grpc

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// 本檔守 #2911：58A §B07「買家面不出現『違規』二字」。
//
// 🔴 **陰性斷言的標的是推導出來的，不是列舉出來的。**
//
// 第一版掃的是序列化後 payload 裡的「違規」二字，那對 `DECIDED_VIOLATION` 這種
// **ASCII enum 值**完全失明——中文只在前端查 CodeMeta 對照表時才出現，wire 上一個
// 中文字都沒有。第一版靠「再補一條字面值檢查 `DECIDED_VIOLATION`」補救，於是
// `VIOLATION_CONFIRMED`（同一個 CodeMeta 檔裡的**另一個**含「違規」label）就漏了，
// 而它正是買家實際看到那四個字的路徑。
//
// ⇒ 判準改成：**從 CodeMeta 註冊表推導出「所有 label 含『違規』的 (enum, value)」**，
// 再斷言那些 value 與 label 都不出現在買家視角的任何回應。後端日後新增第三個含
// 「違規」的 label 時，本測試會自己把它納入，不必有人記得回來改這一支。

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

const (
	bvComplaintID int64 = 6900
	bvStoreID     int64 = 2685914682
)

// bvPoisonedEvents 是「最壞情況」的時間軸：三列都由平台或賣家寫，其中兩列的自由文字
// 含有那兩個字。ADM 的 reason 是端點 60 的**必填**欄，而 DECIDE_VIOLATION 的理由
// 就是「為什麼判違規」⇒ 實務上幾乎必然命中。
func bvPoisonedEvents() []domain.ComplaintEvent {
	s := func(v string) *string { return &v }
	return []domain.ComplaintEvent{
		{
			ID: 1, ComplaintID: bvComplaintID,
			EventType: codemeta.InvoiceComplaintEventTypeSellerReply.Value,
			Action:    s(codemeta.InvoiceComplaintActionResent.Value),
			ActorType: "SELLER", Message: s("已重新寄送到您填的信箱"),
			OccurredAt: xNow.Add(-3 * time.Hour),
		},
		{
			ID: 2, ComplaintID: bvComplaintID,
			EventType: codemeta.InvoiceComplaintEventTypePlatformDecision.Value,
			Action:    s(codemeta.InvoiceComplaintActionReminded.Value),
			ActorType: complaintActorAdmin, Message: s("賣家已逾期，再一次就構成違規，先行提醒"),
			OccurredAt: xNow.Add(-2 * time.Hour),
		},
		{
			ID: 3, ComplaintID: bvComplaintID,
			EventType: codemeta.InvoiceComplaintEventTypePlatformDecision.Value,
			Action:    s(codemeta.InvoiceComplaintActionDecidedViolation.Value),
			ActorType: complaintActorAdmin, Message: s("查證屬實，判定賣家違規並計入 90 天窗口"),
			OccurredAt: xNow.Add(-time.Hour),
		},
	}
}

type bvComplaints struct {
	domain.ComplaintRepository
	rows   map[int64]domain.Complaint
	events []domain.ComplaintEvent
}

func (r *bvComplaints) GetForBuyer(_ context.Context, id, buyerID int64) (domain.Complaint, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID {
		return domain.Complaint{}, pkgerrors.NewBusiness(bizcode.InvoiceComplaintNotFound,
			bizcode.Message(bizcode.InvoiceComplaintNotFound))
	}
	return row, nil
}

func (r *bvComplaints) GetForSeller(_ context.Context, id, storeID int64) (domain.Complaint, error) {
	row, ok := r.rows[id]
	if !ok || storeID != bvStoreID {
		return domain.Complaint{}, pkgerrors.NewBusiness(bizcode.InvoiceComplaintNotFound,
			bizcode.Message(bizcode.InvoiceComplaintNotFound))
	}
	return row, nil
}

func (r *bvComplaints) ListForBuyerOrder(_ context.Context, in domain.BuyerComplaintQuery) ([]domain.Complaint, error) {
	out := []domain.Complaint{}
	for _, row := range r.rows {
		if row.ComplainantBuyerID == in.BuyerID && row.OrderNo != nil && *row.OrderNo == in.OrderNo {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *bvComplaints) CountForBuyerOrder(ctx context.Context, in domain.BuyerComplaintQuery) (int64, error) {
	rows, err := r.ListForBuyerOrder(ctx, in)
	return int64(len(rows)), err
}

func (r *bvComplaints) ListEvents(context.Context, int64) ([]domain.ComplaintEvent, error) {
	return r.events, nil
}

func (r *bvComplaints) CountWithdrawnByDocument(context.Context, int64) (int64, error) { return 0, nil }

func (r *bvComplaints) AppendEvent(_ context.Context, in domain.NewComplaintEvent) (domain.ComplaintEvent, error) {
	return domain.ComplaintEvent{ComplaintID: in.ComplaintID, EventType: in.EventType, ActorType: in.ActorType}, nil
}

func (r *bvComplaints) EscalateByBuyer(_ context.Context, id, buyerID, version int64, _ time.Time) (bool, error) {
	return r.cas(id, buyerID, version, codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
		codemeta.InvoiceComplaintStatusSellerResponded.Value)
}

func (r *bvComplaints) Withdraw(_ context.Context, id, buyerID, version int64, _ time.Time) (bool, error) {
	return r.cas(id, buyerID, version, codemeta.InvoiceComplaintStatusWithdrawn.Value,
		codemeta.InvoiceComplaintStatusOpen.Value, codemeta.InvoiceComplaintStatusOverdue.Value,
		codemeta.InvoiceComplaintStatusSellerResponded.Value)
}

func (r *bvComplaints) Reopen(_ context.Context, id, buyerID, version int64, _, _ time.Time) (bool, error) {
	return r.cas(id, buyerID, version, codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusResolved.Value)
}

// cas 模仿 SQL 的 allowed-from ＋ 版本述詞：不符即 rows=0（回 false），與正式路徑同形。
func (r *bvComplaints) cas(id, buyerID, version int64, next string, allowedFrom ...string) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID || row.Version != version {
		return false, nil
	}
	matched := false
	for _, s := range allowedFrom {
		if row.Status == s {
			matched = true
		}
	}
	if !matched {
		return false, nil
	}
	row.Status = next
	row.Version++
	r.rows[id] = row
	return true, nil
}

func (r *bvComplaints) Create(_ context.Context, in domain.NewComplaint) (domain.Complaint, bool, error) {
	order := in.OrderNo
	row := domain.Complaint{
		ID: in.ID, DocumentID: in.DocumentID, IssuerID: in.IssuerID, OrderNo: &order,
		ComplainantBuyerID: in.ComplainantBuyerID, ReasonCode: in.ReasonCode,
		Status: codemeta.InvoiceComplaintStatusOpen.Value, SlaDueAt: in.SlaDueAt,
		CreatedAt: in.Now, UpdatedAt: in.Now,
	}
	r.rows[row.ID] = row
	return row, true, nil
}

// bvServer 架一台同時掛著買家寫入面（usecase.Complaint）與讀取面（usecase.Query）的 server。
func bvServer(status string, version int64) *InvoiceServer {
	order := xOrder
	repo := &bvComplaints{
		rows: map[int64]domain.Complaint{bvComplaintID: {
			ID: bvComplaintID, DocumentID: xDocID, IssuerID: 5101, OrderNo: &order,
			ComplainantBuyerID: xBuyerA, ReasonCode: "NOT_RECEIVED", Status: status,
			SlaDueAt: xNow.Add(72 * time.Hour), Version: version,
			CreatedAt: xNow.Add(-24 * time.Hour), UpdatedAt: xNow.Add(-time.Hour),
		}},
		events: bvPoisonedEvents(),
	}
	synced := xNow
	clock := func() time.Time { return xNow }
	var seq int64 = 9200000000

	uc := usecase.NewComplaint(xTx{}, repo, xDocs{owner: xBuyerA}, xIssuers{}, xOutbox{},
		xBiz{synced: &synced}, nil, func() int64 { seq++; return seq }, nil).WithClock(clock)
	q := usecase.NewQuery(nil, nil, nil, nil, nil, nil, repo, nil, nil, nil, nil,
		xBiz{synced: &synced}, nil, nil, qCancelled{}, usecase.IssuanceConfig{}, nil).WithClock(clock)

	return NewInvoiceServer(nil, q, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// ---------------------------------------------------------------------------
// 買家面 RPC 覆蓋表（本檔與下方的 protoreflect 閘門共用同一份）
// ---------------------------------------------------------------------------

type bvCall struct {
	// status/version 是這支 RPC 的守衛要求的來源狀態。
	status  string
	version int64
	invoke  func(*InvoiceServer) (proto.Message, error)
}

func bvBuyerCalls() map[string]bvCall {
	ctx := context.Background()
	id := "6900"
	return map[string]bvCall{
		// 端點 13：走 9310 分支——它回的是**既有那件案子的完整單筆**，
		// 是建案路徑上唯一帶得出 ADM 事件的出口（新建案只有買家自己那一列）。
		"CreateInvoiceComplaint": {
			status: codemeta.InvoiceComplaintStatusOpen.Value, version: 0,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.CreateInvoiceComplaint(ctx, &pb.CreateInvoiceComplaintRequest{
					BuyerId: xBuyerA, OrderNo: xOrder, ReasonCode: "NOT_RECEIVED",
				})
			},
		},
		// 端點 14
		"ListOrderInvoiceComplaints": {
			status: codemeta.InvoiceComplaintStatusViolationConfirmed.Value, version: 3,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.ListOrderInvoiceComplaints(ctx, &pb.ListOrderInvoiceComplaintsRequest{
					BuyerId: xBuyerA, OrderNo: xOrder,
				})
			},
		},
		// 端點 15
		"GetInvoiceComplaint": {
			status: codemeta.InvoiceComplaintStatusViolationConfirmed.Value, version: 3,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
					BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id,
				})
			},
		},
		// 端點 63
		"RejectInvoiceComplaintResponse": {
			status: codemeta.InvoiceComplaintStatusSellerResponded.Value, version: 2,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.RejectInvoiceComplaintResponse(ctx, &pb.RejectInvoiceComplaintResponseRequest{
					BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id,
					ExpectedVersion: "2", Reason: "還是沒收到",
				})
			},
		},
		// 端點 64
		"WithdrawInvoiceComplaint": {
			status: codemeta.InvoiceComplaintStatusOpen.Value, version: 2,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.WithdrawInvoiceComplaint(ctx, &pb.WithdrawInvoiceComplaintRequest{
					BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id, ExpectedVersion: "2",
				})
			},
		},
		// 端點 65
		"ReopenInvoiceComplaint": {
			status: codemeta.InvoiceComplaintStatusResolved.Value, version: 2,
			invoke: func(s *InvoiceServer) (proto.Message, error) {
				return s.ReopenInvoiceComplaint(ctx, &pb.ReopenInvoiceComplaintRequest{
					BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id, ExpectedVersion: "2",
				})
			},
		},
	}
}

// ---------------------------------------------------------------------------
// 從 CodeMeta 推導「不得出現在買家面的 (enum, value)」
// ---------------------------------------------------------------------------

// forbiddenBuyerPhrase 是 58A §B07／§E3 明訂不得出現在買家面的字。
//
// 它是**推導的輸入**，不是斷言清單本身：下面的 codemetaEntriesWithPhrase 用它去掃
// CodeMeta 註冊表，掃出哪些碼值的中文 label 含有它。
const forbiddenBuyerPhrase = "違規"

// codemetaEntry 是 CodeMeta 註冊表裡的一個成員。
type codemetaEntry struct {
	Enum  string // 型別名，例：InvoiceComplaintStatus
	Const string // Go 常數名，例：InvoiceComplaintStatusViolationConfirmed
	Value string // wire 上會出現的值，例：VIOLATION_CONFIRMED
	Label string // 前端會印出來的中文，例：判定賣家違規
}

func (e codemetaEntry) String() string {
	return fmt.Sprintf("%s.%s（value=%q label=%q）", e.Enum, e.Const, e.Value, e.Label)
}

// bvRepoRoot 從本 package 往上找到 repo 根（以 go.work 為錨）。形狀沿用
// internal/mq/producer_contract_test.go 的同名 helper。
func bvRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("找不到 repo root（往上 12 層都沒有 go.work）")
	return ""
}

// codemetaEntriesWithPhrase 推導「所有 label 含 phrase 的 CodeMeta 成員」。
//
// # 為什麼用解析原始碼，不用反射
//
// Go 的反射列舉不到 package 層級的 var，而 `pkg/codemeta` 是逐 enum 各自一組
// `var (...)` ＋ `All<Name>s` 切片的生成碼，沒有任何跨 enum 的總索引。要做到
// 「後端新增第三個含『違規』的 label 時測試自己跟上」，只能從那份生成檔本身推導。
//
// 生成檔是 CodeMeta Markdown（SSOT）的直接產物（§8.5：改 .md → generate.sh），
// 而它同時就是**本服務編譯時真正連進去的那一份** ⇒ 解析它等價於解析 SSOT，
// 且不會有「解析了一份沒人用的副本」那種假綠。下面的
// TestViolationWordedCodeMetaEntriesAreDerivedNotEnumerated 會把解析結果與編譯進來的
// 常數逐字對拍，把這件事釘住。
//
// 兩個檔都掃：
//   - registry.go：`X = EnumType{Code: ..., Value: ..., Label: ...}` 形狀（UPPER_SNAKE 值）
//   - labels.go：`var xxxLabels = map[string]string{"4001": "草稿"}` 形狀（數字碼值）
func codemetaEntriesWithPhrase(t *testing.T, phrase string) []codemetaEntry {
	t.Helper()
	root := bvRepoRoot(t)

	fromRegistry := parseCodemetaRegistry(t, filepath.Join(root, "pkg", "codemeta", "registry.go"), phrase)
	fromLabels := parseCodemetaLabelMaps(t, filepath.Join(root, "pkg", "codemeta", "labels.go"), phrase)
	out := make([]codemetaEntry, 0, len(fromRegistry)+len(fromLabels))
	out = append(out, fromRegistry...)
	out = append(out, fromLabels...)

	sort.Slice(out, func(i, j int) bool {
		if out[i].Enum != out[j].Enum {
			return out[i].Enum < out[j].Enum
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// parseCodemetaRegistry 解 registry.go 的 struct 常數。
func parseCodemetaRegistry(t *testing.T, path, phrase string) []codemetaEntry {
	t.Helper()
	file := bvParseGoFile(t, path)

	var out []codemetaEntry
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, val := range vs.Values {
				lit, ok := val.(*ast.CompositeLit)
				if !ok {
					continue
				}
				enum, ok := lit.Type.(*ast.Ident)
				if !ok {
					continue // 切片（All<Name>s）等其他形狀
				}
				fields := bvStructFields(lit)
				if !strings.Contains(fields["Label"], phrase) {
					continue
				}
				name := ""
				if i < len(vs.Names) {
					name = vs.Names[i].Name
				}
				out = append(out, codemetaEntry{
					Enum: enum.Name, Const: name,
					Value: fields["Value"], Label: fields["Label"],
				})
			}
		}
	}
	return out
}

// parseCodemetaLabelMaps 解 labels.go 的 `map[string]string` 對照表。
//
// 這一批的 value 是數字碼（"4001"）。它們今天一個都不含「違規」，掃它是為了讓
// 「後端新增含違規的 label」這件事在**任何一種 CodeMeta 形狀**下都被納入。
func parseCodemetaLabelMaps(t *testing.T, path, phrase string) []codemetaEntry {
	t.Helper()
	file := bvParseGoFile(t, path)

	var out []codemetaEntry
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, val := range vs.Values {
				lit, ok := val.(*ast.CompositeLit)
				if !ok {
					continue
				}
				if _, ok := lit.Type.(*ast.MapType); !ok {
					continue
				}
				enum := ""
				if i < len(vs.Names) {
					enum = strings.TrimSuffix(vs.Names[i].Name, "Labels")
				}
				for _, elt := range lit.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, kok := bvStringLit(kv.Key)
					label, lok := bvStringLit(kv.Value)
					if !kok || !lok || !strings.Contains(label, phrase) {
						continue
					}
					out = append(out, codemetaEntry{
						Enum: enum, Const: enum + "/" + key, Value: key, Label: label,
					})
				}
			}
		}
	}
	return out
}

func bvParseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		// fail-loud：檔案搬家／改名時，這支測試的推導基礎沒了，不該靜靜地推導出空集合。
		t.Fatalf("讀不到 CodeMeta 生成檔 %s：%v\n"+
			"（推導不到標的時本測試會恆綠，所以這裡必須 fail 而不是回空）", path, err)
	}
	return file
}

// bvStructFields 把 `T{A: "x", B: "y"}` 的字串欄位攤成 map。
func bvStructFields(lit *ast.CompositeLit) map[string]string {
	out := map[string]string{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if s, ok := bvStringLit(kv.Value); ok {
			out[key.Name] = s
		}
	}
	return out
}

func bvStringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// ---------------------------------------------------------------------------
// 推導本身的鑑別力
// ---------------------------------------------------------------------------

// 這一支釘住「推導器壞掉會讓陰性斷言恆綠」那個 bug class。三件事一起驗：
//
//	① 推導結果非空（路徑打錯／生成檔換形狀 ⇒ 空集合 ⇒ 下面每一條 for 迴圈都不執行）
//	② 兩個**已知**的成員都在裡面（少一個 ⇒ 解析器漏看了某一段）
//	③ 解析出來的 label 與**編譯進來的常數**逐字相同 ⇒ 證明解析的是同一份檔案，
//	   不是某個沒人用的副本（同源教訓：memory `feedback_no_blind_measurement`）
//
// ⚠️ 它**不是**一張把數量釘死的 pin。新增第三個含「違規」的 label 時本測試照樣綠，
// 而那個新成員會自動出現在下面陰性斷言的標的裡——那正是本次要換掉列舉式判準的理由。
func TestViolationWordedCodeMetaEntriesAreDerivedNotEnumerated(t *testing.T) {
	entries := codemetaEntriesWithPhrase(t, forbiddenBuyerPhrase)
	if len(entries) == 0 {
		t.Fatalf("從 CodeMeta 推導出 0 個含「%s」的 label —— 推導器壞了，"+
			"而它一壞，本檔所有陰性斷言都會恆綠", forbiddenBuyerPhrase)
	}
	for _, e := range entries {
		t.Logf("推導到：%s", e)
	}

	// ② ＋ ③：兩個已知成員，以編譯進來的常數為對照。
	known := []codemetaEntry{
		{
			Enum: "InvoiceComplaintStatus", Const: "InvoiceComplaintStatusViolationConfirmed",
			Value: codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
			Label: codemeta.InvoiceComplaintStatusViolationConfirmed.Label,
		},
		{
			Enum: "InvoiceComplaintAction", Const: "InvoiceComplaintActionDecidedViolation",
			Value: codemeta.InvoiceComplaintActionDecidedViolation.Value,
			Label: codemeta.InvoiceComplaintActionDecidedViolation.Label,
		},
	}
	for _, want := range known {
		if !strings.Contains(want.Label, forbiddenBuyerPhrase) {
			t.Fatalf("CodeMeta 的 %s label 已不含「%s」——本檔的陰性斷言失去標的，"+
				"請回頭確認 58A §B07 是否仍需要投影", want.Const, forbiddenBuyerPhrase)
		}
		found := false
		for _, got := range entries {
			if got.Enum != want.Enum || got.Value != want.Value {
				continue
			}
			found = true
			if got.Label != want.Label {
				t.Fatalf("解析到的 label %q 與編譯進來的常數 %q 不同 —— "+
					"解析的不是同一份檔案（副本？舊版？）", got.Label, want.Label)
			}
			if got.Const != want.Const {
				t.Errorf("常數名解析成 %q，want %q", got.Const, want.Const)
			}
		}
		if !found {
			t.Fatalf("推導結果缺少 %s —— 解析器漏看了 registry.go 的某一段。\n實得：%v",
				want, entries)
		}
	}
}

// ---------------------------------------------------------------------------
// wire 上的字串葉節點
// ---------------------------------------------------------------------------

// wireLeaf 是回應裡的一個字串（或 bytes）葉節點，帶欄位路徑供錯誤訊息定位。
type wireLeaf struct {
	Path  string
	Value string
}

// collectWireLeaves 走訪整棵 message，收集**所有深度**的字串葉節點。
//
// 為什麼不只掃 proto.Marshal 的位元組：那個做法對 enum **值**只能做子字串比對，而
// `RESOLVED` 是 `DECIDED_RESOLVED` 的子字串 ⇒ 一旦推導出的標的是個短值就會誤報。
// 走訪型別樹拿得到「一整格欄位的值」，可以用**完全相等**判定 enum 值、用子字串判定
// 中文 label，兩種語意各用各的判準。新增欄位不必回頭改這裡。
func collectWireLeaves(m protoreflect.Message, prefix string, out *[]wireLeaf) {
	if m == nil || !m.IsValid() {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := prefix + string(fd.Name())
		switch {
		case fd.IsMap():
			v.Map().Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
				kp := fmt.Sprintf("%s[%v]", name, k.Interface())
				if fd.MapKey().Kind() == protoreflect.StringKind {
					*out = append(*out, wireLeaf{Path: kp + ".<key>", Value: k.String()})
				}
				appendWireLeaf(fd.MapValue(), mv, kp, out)
				return true
			})
		case fd.IsList():
			l := v.List()
			for i := 0; i < l.Len(); i++ {
				appendWireLeaf(fd, l.Get(i), fmt.Sprintf("%s[%d]", name, i), out)
			}
		default:
			appendWireLeaf(fd, v, name, out)
		}
		return true
	})
}

func appendWireLeaf(fd protoreflect.FieldDescriptor, v protoreflect.Value, path string, out *[]wireLeaf) {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		collectWireLeaves(v.Message(), path+".", out)
	case protoreflect.StringKind:
		*out = append(*out, wireLeaf{Path: path, Value: v.String()})
	case protoreflect.BytesKind:
		*out = append(*out, wireLeaf{Path: path, Value: string(v.Bytes())})
	}
}

// ---------------------------------------------------------------------------
// 陰性：買家在任何路徑上都拿不到那兩個字
// ---------------------------------------------------------------------------

// 🔴 本檔最重要的一支。標的來自 CodeMeta 推導，斷言分三層：
//
//	A. 序列化後的**整個 payload** 不含「違規」二字 —— 涵蓋任何深度、任何欄位，
//	   連 unknown fields 都在射程內（label 若以中文形式外洩必被抓）
//	B. 任何字串葉節點都不**包含**推導出的 label —— 自由文字裡夾帶那句話也算洩漏
//	C. 任何字串葉節點都不**等於**推導出的 value —— 這一層才是 ASCII enum 值的守衛，
//	   也是第一版整個缺掉的那一層
//
// ⚠️ 六支 RPC 對三個投影軸的覆蓋不均，這是狀態機決定的、不是漏測：
// `status` 軸只有端點 13（9310 分支）／14／15 帶得出 VIOLATION_CONFIRMED，因為它是
// 終態、**沒有任何買家寫入邊從它出發**（63 要 SELLER_RESPONDED、64 要 OPEN 系、
// 65 要 RESOLVED）⇒ 63／64／65 不可能回一個 status=VIOLATION_CONFIRMED 的視圖。
// 而 `action`／`note` 兩軸六支全部覆蓋（時間軸恆帶 ADM 的裁定列）。
func TestBuyerComplaintPayloadNeverLeaksViolationWording(t *testing.T) {
	entries := codemetaEntriesWithPhrase(t, forbiddenBuyerPhrase)
	if len(entries) == 0 {
		t.Fatal("推導出 0 個標的，本測試會恆綠 —— 先修 codemetaEntriesWithPhrase")
	}

	for name, tc := range bvBuyerCalls() {
		t.Run(name, func(t *testing.T) {
			resp, err := tc.invoke(bvServer(tc.status, tc.version))
			if err != nil {
				t.Fatalf("%s 回了 gRPC error：%v", name, err)
			}

			raw, err := proto.Marshal(resp)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			// A
			if bytes.Contains(raw, []byte(forbiddenBuyerPhrase)) {
				t.Fatalf("🔴 58A §B07：買家面不得出現「%s」二字，但 %s 的回應裡有。\n完整回應：%+v",
					forbiddenBuyerPhrase, name, resp)
			}

			var leaves []wireLeaf
			collectWireLeaves(resp.ProtoReflect(), "", &leaves)
			if len(leaves) == 0 {
				t.Fatalf("%s 的回應走訪出 0 個字串葉節點 —— B／C 兩層會恆綠", name)
			}

			for _, e := range entries {
				for _, lf := range leaves {
					// B
					if strings.Contains(lf.Value, e.Label) {
						t.Fatalf("🔴 %s 的 %s = %q 含 CodeMeta label「%s」（%s）",
							name, lf.Path, lf.Value, e.Label, e)
					}
					// C
					if lf.Value == e.Value {
						t.Fatalf("🔴 %s 的 %s = %q —— 前端照 58A §0.4 ⑤ 查 CodeMeta 會印出「%s」。\n"+
							"標的來源：%s\n完整回應：%+v",
							name, lf.Path, lf.Value, e.Label, e, resp)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 陽性：投影不是「整列不下發」，也不是「把資訊銷毀」
// ---------------------------------------------------------------------------

// 依 58A §B07 版面結構逐字「角色（賣家／**平台**）」：平台那一列必須還在，
// 只是動作被投影成對買家安全的那一個。整列刪掉會讓案件從「平台處理中」跳到
// 「申訴已完成」而歷程上沒有任何平台紀錄。
func TestBuyerComplaintKeepsPlatformRowsWithSafeAction(t *testing.T) {
	s := bvServer(codemeta.InvoiceComplaintStatusViolationConfirmed.Value, 3)
	resp, err := s.GetInvoiceComplaint(context.Background(), &pb.GetInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6900",
	})
	if err != nil {
		t.Fatalf("GetInvoiceComplaint: %v", err)
	}
	rows := resp.GetData().GetResponses()
	if len(rows) != len(bvPoisonedEvents()) {
		t.Fatalf("歷程列數 = %d，want %d —— 投影不得刪列", len(rows), len(bvPoisonedEvents()))
	}

	var admin, seller int
	for _, r := range rows {
		switch r.GetActor() {
		case complaintActorAdmin:
			admin++
			if r.GetNote() != "" {
				t.Fatalf("ADM 的裁決理由不得下發給買家，實得 %q", r.GetNote())
			}
		case "SELLER":
			seller++
			// 賣家寫給買家的回覆是 B07「請查看下方回覆內容」的內容本身，不得一併清掉。
			if r.GetNote() == "" {
				t.Fatal("賣家的回覆內容被清掉了——投影的範圍只有 ADM 的列")
			}
		}
	}
	if admin != 2 || seller != 1 {
		t.Fatalf("actor 分布 = ADM %d／SELLER %d，want 2／1", admin, seller)
	}

	last := rows[len(rows)-1]
	if last.GetAction() != codemeta.InvoiceComplaintActionDecidedResolved.Value {
		t.Fatalf("DECIDED_VIOLATION 應投影成 %s，實得 %q",
			codemeta.InvoiceComplaintActionDecidedResolved.Value, last.GetAction())
	}

	// status 投影成 RESOLVED：58A §B07 狀態表 VIOLATION_CONFIRMED 列的「買家面語意」
	// 逐字就是「已解決」，而那正是 CodeMeta 對 RESOLVED 的 label。
	if got := resp.GetData().GetStatus(); got != codemeta.InvoiceComplaintStatusResolved.Value {
		t.Fatalf("status 應投影成 %s，實得 %q",
			codemeta.InvoiceComplaintStatusResolved.Value, got)
	}
}

// 🔴 投影 status 之後，「有沒有重開邊」的唯一判準落在 reopen_allowed_after 上。
// 這一支釘住它：終態案件被投影成 RESOLVED，但**不得**因此獲得一個重開時間，
// 否則前端 §B07 共通規則第 4 條的重開鍵判斷會對終態案件開出一顆按得下去的鍵。
//
// 它同時守住 toComplaintInfo 與投影的**順序**：先投影再組裝的話，
// `c.Status == complaintStatusResolved` 會成立、這一欄會被填上值，本測試轉紅。
func TestBuyerViolationCaseCarriesNoReopenWindow(t *testing.T) {
	s := bvServer(codemeta.InvoiceComplaintStatusViolationConfirmed.Value, 3)
	resp, err := s.GetInvoiceComplaint(context.Background(), &pb.GetInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6900",
	})
	if err != nil {
		t.Fatalf("GetInvoiceComplaint: %v", err)
	}
	if got := resp.GetData().GetReopenAllowedAfter(); got != "" {
		t.Fatalf("VIOLATION_CONFIRMED 是終態、K09 對 reopen_allowed_after 寫 NULL，"+
			"買家面卻拿到 %q —— 前端會渲染一顆按得下去的重開鍵", got)
	}
}

// 賣家與 ADM 的視角不套投影：這條是「資訊沒有被銷毀、只是不給買家」的證據。
// 少了它，把 responses[] 整個清空、把 status 改成別的值，也會讓陰性測試變綠。
func TestSellerComplaintViewStillCarriesTheRuling(t *testing.T) {
	s := bvServer(codemeta.InvoiceComplaintStatusViolationConfirmed.Value, 3)
	resp, err := s.GetSellerInvoiceComplaint(context.Background(), &pb.GetSellerInvoiceComplaintRequest{
		StoreId: bvStoreID, ComplaintId: "6900",
	})
	if err != nil {
		t.Fatalf("GetSellerInvoiceComplaint: %v", err)
	}
	if got := resp.GetData().GetStatus(); got != codemeta.InvoiceComplaintStatusViolationConfirmed.Value {
		t.Fatalf("賣家視角的 status 必須是真實值 %s，實得 %q —— 投影只針對買家",
			codemeta.InvoiceComplaintStatusViolationConfirmed.Value, got)
	}
	raw, err := proto.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(codemeta.InvoiceComplaintActionDecidedViolation.Value)) {
		t.Fatal("賣家視角必須看得到真實裁定（DECIDED_VIOLATION），投影只針對買家")
	}
	if !bytes.Contains(raw, []byte("查證屬實")) {
		t.Fatal("賣家視角必須看得到 ADM 的裁決理由")
	}
}

// ---------------------------------------------------------------------------
// 閘門：新增買家面申訴 RPC 而沒套投影時，本測試轉紅
// ---------------------------------------------------------------------------

// 判定不靠命名也不靠人記得：從 proto 描述子推導「入參帶 buyer_id 且回應可遞移到達
// InvoiceComplaintInfo」的 RPC 全集，再與上面那張覆蓋表對帳。
//
// ⚠️ 標的是 **InvoiceComplaintInfo 不是 ComplaintResponseItem**：三個投影軸裡有兩個
// （action／note）住在 responses[]，但 `status` 住在 Info 本身 ⇒ 錨在 Item 上的話，
// 一支只回狀態、不回歷程的新 RPC 會整支落在閘門外。錨在 Info 上是兩者的超集。
//
// 為什麼需要它：漏套投影是**完全靜默**的——編譯過、既有測試全綠、畫面正常，
// 只有真的有一件 DECIDE_VIOLATION 落地時才會外顯，而那時已經在 prod。
func TestEveryBuyerFacingComplaintRPCIsCoveredByTheProjectionTest(t *testing.T) {
	svc := pb.File_invoice_invoice_proto.Services().ByName("InvoiceService")
	if svc == nil {
		t.Fatal("找不到 InvoiceService 描述子")
	}

	var derived []string
	for i := 0; i < svc.Methods().Len(); i++ {
		m := svc.Methods().Get(i)
		if m.Input().Fields().ByName("buyer_id") == nil {
			continue
		}
		if !bvReachesComplaintInfo(m.Output(), map[protoreflect.FullName]bool{}) {
			continue
		}
		derived = append(derived, string(m.Name()))
	}
	sort.Strings(derived)

	covered := make([]string, 0, len(bvBuyerCalls()))
	for name := range bvBuyerCalls() {
		covered = append(covered, name)
	}
	sort.Strings(covered)

	if len(derived) == 0 {
		t.Fatal("推導出 0 支買家面申訴 RPC —— 判定條件本身壞了（bvReachesComplaintInfo 或 buyer_id 欄名）")
	}
	if bvJoin(derived) != bvJoin(covered) {
		t.Fatalf("🔴 買家面申訴 RPC 與投影測試的覆蓋表不一致。\nproto 推導：%v\n測試覆蓋：%v\n"+
			"新增的那一支必須 ① 用 toBuyerComplaintInfo 組回應 ② 加進 bvBuyerCalls()", derived, covered)
	}
}

func bvJoin(v []string) string {
	out := ""
	for _, s := range v {
		out += s + "\n"
	}
	return out
}

// bvReachesComplaintInfo 判斷一個 message 能不能遞移到達 InvoiceComplaintInfo。
func bvReachesComplaintInfo(md protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) bool {
	if md == nil || seen[md.FullName()] {
		return false
	}
	seen[md.FullName()] = true
	if md.FullName() == "hawkr.invoice.InvoiceComplaintInfo" {
		return true
	}
	for i := 0; i < md.Fields().Len(); i++ {
		f := md.Fields().Get(i)
		if f.Kind() != protoreflect.MessageKind && f.Kind() != protoreflect.GroupKind {
			continue
		}
		if bvReachesComplaintInfo(f.Message(), seen) {
			return true
		}
	}
	return false
}
