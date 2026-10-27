package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/client/buyer"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

type fakeBuyerNames struct {
	byID map[int64]buyer.Display
	err  error
	got  []int64
}

func (f *fakeBuyerNames) BatchGetBuyerDisplay(_ context.Context, ids []int64) (map[int64]buyer.Display, error) {
	f.got = ids
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]buyer.Display{}
	for _, id := range ids {
		if d, ok := f.byID[id]; ok {
			out[id] = d
		}
	}
	return out, nil
}

func complaintView(buyerID int64) usecase.ComplaintView {
	return usecase.ComplaintView{Complaint: domain.Complaint{ComplainantBuyerID: buyerID}}
}

// 🔴 本檔的核心一格：**出去的必須是遮罩後的值**。
//
// 讀這一欄的是賣家與平台管理員，不是資料主體本人（58A §0.4 ③）。
// 這一層若回明碼，遮罩就得由每個呼叫端各自記得做——而「忘了做」不會有任何症狀。
func TestResolveBuyerNames_MasksBeforeReturning(t *testing.T) {
	r := &fakeBuyerNames{byID: map[int64]buyer.Display{
		11: {DisplayName: "陳小明"},
		12: {DisplayName: "已註銷的買家", Deleted: true},
	}}
	s := &InvoiceServer{buyerNames: r, log: zap.NewNop()}

	got := s.resolveBuyerNames(context.Background(), []int64{11, 12, 13})
	if got[11] != "陳•••" {
		t.Fatalf("🔴 明碼外洩：got %q，應為遮罩後的值", got[11])
	}
	// 墓碑不是個資，遮了會變成讀不懂的字串，而它存在的唯一理由就是要讓人讀懂。
	if got[12] != "已註銷的買家" {
		t.Fatalf("墓碑被遮罩了：%q", got[12])
	}
	if _, ok := got[13]; ok {
		t.Fatalf("查無的 id 不得進 map：%#v", got)
	}
}

func TestResolveBuyerNames_DegradesWithoutFailingTheList(t *testing.T) {
	s := &InvoiceServer{buyerNames: &fakeBuyerNames{err: errors.New("buyer down")}, log: zap.NewNop()}
	if got := s.resolveBuyerNames(context.Background(), []int64{11}); len(got) != 0 {
		t.Fatalf("失敗應回空 map，got=%#v", got)
	}
	s2 := &InvoiceServer{buyerNames: nil, log: zap.NewNop()}
	if got := s2.resolveBuyerNames(context.Background(), []int64{11}); len(got) != 0 {
		t.Fatalf("nil resolver 應回空 map，got=%#v", got)
	}
}

func TestResolveBuyerNames_DedupesAndDropsZero(t *testing.T) {
	r := &fakeBuyerNames{byID: map[int64]buyer.Display{11: {DisplayName: "陳小明"}}}
	s := &InvoiceServer{buyerNames: r, log: zap.NewNop()}
	s.resolveBuyerNames(context.Background(), []int64{11, 11, 0, 12})
	if len(r.got) != 2 || r.got[0] != 11 || r.got[1] != 12 {
		t.Fatalf("去重／濾 0 沒生效：%#v", r.got)
	}
}

// 🔴 第二格核心：**名字要掛在對的那一列上。**
//
// records 與 views 同序，錯位的症狀不是報錯，是甲買家的名字出現在乙買家的申訴卡上
// ——那是一次跨使用者的資料外洩，而畫面完全正常。
func TestFillBuyerNames_AttachesToTheRightRow(t *testing.T) {
	// 🔴 兩個名字的**第一個字必須不同**：遮罩只保留第一個字，
	// 用「阿明／阿華」會雙雙遮成「阿•••」⇒ 錯位了測試照樣綠，等於沒驗到排序。
	r := &fakeBuyerNames{byID: map[int64]buyer.Display{
		11: {DisplayName: "陳小明"},
		22: {DisplayName: "林大華"},
	}}
	s := &InvoiceServer{buyerNames: r, log: zap.NewNop()}

	records := []*pb.InvoiceComplaintInfo{
		{ComplaintId: "A"}, {ComplaintId: "B"}, {ComplaintId: "C"},
	}
	views := []usecase.ComplaintView{
		complaintView(22), complaintView(11), complaintView(99), // 99 查無
	}
	s.fillBuyerNames(context.Background(), records, views)

	if records[0].GetBuyerMaskedName() != "林•••" {
		t.Fatalf("第 1 列應是 buyer 22（林大華）的名字，got %q", records[0].GetBuyerMaskedName())
	}
	if records[1].GetBuyerMaskedName() != "陳•••" {
		t.Fatalf("第 2 列應是 buyer 11（陳小明）的名字，got %q", records[1].GetBuyerMaskedName())
	}
	if records[2].GetBuyerMaskedName() != "" {
		t.Fatalf("查無的買家應留空字串，got %q", records[2].GetBuyerMaskedName())
	}
}

// 長度不一致時整批放棄，不得靠索引硬對——那正好是上一格要防的錯位。
func TestFillBuyerNames_RefusesMismatchedLengths(t *testing.T) {
	r := &fakeBuyerNames{byID: map[int64]buyer.Display{11: {DisplayName: "阿明"}}}
	s := &InvoiceServer{buyerNames: r, log: zap.NewNop()}
	records := []*pb.InvoiceComplaintInfo{{ComplaintId: "A"}, {ComplaintId: "B"}}
	s.fillBuyerNames(context.Background(), records, []usecase.ComplaintView{complaintView(11)})
	for i, rec := range records {
		if rec.GetBuyerMaskedName() != "" {
			t.Fatalf("長度不符時不得填任何一列，第 %d 列 = %q", i, rec.GetBuyerMaskedName())
		}
	}
	if r.got != nil {
		t.Fatal("長度不符時不該發出 RPC")
	}
}

func TestFillOneBuyerName(t *testing.T) {
	r := &fakeBuyerNames{byID: map[int64]buyer.Display{11: {DisplayName: "alexis"}}}
	s := &InvoiceServer{buyerNames: r, log: zap.NewNop()}
	rec := &pb.InvoiceComplaintInfo{}
	s.fillOneBuyerName(context.Background(), rec, complaintView(11))
	if rec.GetBuyerMaskedName() != "a•••" {
		t.Fatalf("單筆版沒填或沒遮罩：%q", rec.GetBuyerMaskedName())
	}
}
