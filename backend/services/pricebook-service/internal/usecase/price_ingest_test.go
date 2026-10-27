package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func TestRecordDigestIsStable(t *testing.T) {
	r := moa.TradeRecord{
		TransDate: "115.07.26", CropCode: "LA1", MarketCode: "104",
		AvgPrice: 46.2, UpperPrice: 54.6, MiddlePrice: 47.5, LowerPrice: 34.1,
		TransQuantity: 10957,
	}

	first := RecordDigest(r)
	second := RecordDigest(r)
	if first != second {
		t.Error("同一筆資料的 digest 不穩定")
	}
}

func TestRecordDigestChangesWhenPriceRevised(t *testing.T) {
	base := moa.TradeRecord{TransDate: "115.07.26", CropCode: "LA1", MarketCode: "104", AvgPrice: 46.2}
	revised := base
	revised.AvgPrice = 47.0
	if RecordDigest(base) == RecordDigest(revised) {
		t.Error("價格修訂後 digest 應改變，否則修訂偵測失效")
	}
}

func TestRecordDigestIgnoresCropNameChange(t *testing.T) {

	base := moa.TradeRecord{TransDate: "115.07.26", CropCode: "LA1", MarketCode: "104", AvgPrice: 46.2, CropName: "甘藍-初秋"}
	renamed := base
	renamed.CropName = "甘藍-初秋 "
	if RecordDigest(base) != RecordDigest(renamed) {
		t.Error("顯示名稱變動不應觸發修訂")
	}
}

func TestNewPriceIngestWithClientInjectsGivenClientAndRetryDelay(t *testing.T) {
	stub := &stubPriceClient{}
	wantDelay := 3 * time.Millisecond
	pi := NewPriceIngestWithClient(stub, nil, nil, wantDelay)
	if pi.client != stub {
		t.Error("NewPriceIngestWithClient 應該把傳入的 client 存進 PriceIngest.client 欄位")
	}
	if pi.retryDelay != wantDelay {
		t.Errorf("NewPriceIngestWithClient 應該原樣存下呼叫端傳入的 retryDelay，不應悄悄退回 emptyResponseRetryDelay 生產預設值，預期 %v，實得 %v", wantDelay, pi.retryDelay)
	}
}

func TestAttentionReasonFiresWhenMarketsEmptyAfterRetry(t *testing.T) {
	s := IngestStats{Fetched: 10, Inserted: 5, EmptyAfterRetry: []EmptyLeafGap{
		{MarketCode: "800", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{MarketCode: "514", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("EmptyAfterRetry 非空時 AttentionReason 應回傳非空字串")
	}
	if !strings.Contains(reason, "800") || !strings.Contains(reason, "514") {
		t.Errorf("AttentionReason 應點名受影響市場代碼，實得: %q", reason)
	}
}

func TestAttentionReasonFiresWhenEveryAttemptedPartitionCameBackEmpty(t *testing.T) {
	s := IngestStats{PartitionsAttempted: 34, Fetched: 0}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("問了 34 個分區卻一列都沒拿到時，AttentionReason 應回傳非空字串")
	}
	if !strings.Contains(reason, "34") {
		t.Errorf("AttentionReason 應點名求取的分區數，實得: %q", reason)
	}
}

func TestAttentionReasonSilentWhenNothingWasDue(t *testing.T) {
	s := IngestStats{PartitionsAttempted: 0, Fetched: 0}
	if reason := s.AttentionReason(); reason != "" {
		t.Errorf("沒有待補分區時不得告警，實得: %q", reason)
	}
}

func TestAttentionReasonSilentWhenSomeRowsCameBack(t *testing.T) {
	s := IngestStats{PartitionsAttempted: 34, Fetched: 120, Inserted: 120}
	if reason := s.AttentionReason(); reason != "" {
		t.Errorf("有拿到列時不得因本條判準告警，實得: %q", reason)
	}
}

func TestAttentionReasonEmptyWhenNoEmptyAfterRetryMarkets(t *testing.T) {
	s := IngestStats{Fetched: 10, Inserted: 5}
	if reason := s.AttentionReason(); reason != "" {
		t.Errorf("沒有 EmptyAfterRetry 市場時 AttentionReason 應為空字串，實得: %q", reason)
	}
}

func TestAttentionReasonOnlySkippedStillFiresAMessage(t *testing.T) {
	s := IngestStats{Fetched: 10, Skipped: 10}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("Skipped>0 且 Inserted+Revised==0 時 AttentionReason 應回傳非空字串")
	}
	if !strings.Contains(reason, "抓到 10 列但一列都沒寫入") {
		t.Errorf("AttentionReason 應保留原本的 Skipped 判準訊息，實得: %q", reason)
	}
}

func TestAttentionReasonDedupesRepeatedMarketCodes(t *testing.T) {
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	s := IngestStats{
		Fetched:  10,
		Inserted: 5,
		EmptyAfterRetry: []EmptyLeafGap{
			{MarketCode: "104", Start: jan, End: jan},
			{MarketCode: "104", Start: feb, End: feb},
			{MarketCode: "108", Start: jan, End: jan},
			{MarketCode: "104", Start: mar, End: mar},
		},
	}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("EmptyAfterRetry 非空時 AttentionReason 應回傳非空字串")
	}
	if !strings.Contains(reason, "2 個市場") {
		t.Errorf("去重後應是 2 個市場（104、108），實得: %q", reason)
	}
	if strings.Contains(reason, "3 個市場") {
		t.Errorf("不應把重複出現的 104 算成獨立市場，實得: %q", reason)
	}
	if got := strings.Count(reason, "104"); got != 1 {
		t.Errorf("市場代碼清單裡 104 應只出現一次（去重），實得出現 %d 次: %q", got, reason)
	}
	if !strings.Contains(reason, "108") {
		t.Errorf("AttentionReason 應仍點名市場 108，實得: %q", reason)
	}
}

func TestAttentionReasonCombinesBothReasonsWhenBothFire(t *testing.T) {
	s := IngestStats{
		Fetched:  11,
		Skipped:  1,
		Inserted: 0,
		Revised:  0,
		EmptyAfterRetry: []EmptyLeafGap{
			{MarketCode: "800", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("Skipped 與 EmptyAfterRetry 判準同時成立時 AttentionReason 應回傳非空字串")
	}
	if !strings.Contains(reason, "抓到 11 列但一列都沒寫入") {
		t.Errorf("合併後的訊息應仍包含 Skipped 判準原文，實得: %q", reason)
	}
	if !strings.Contains(reason, "800") {
		t.Errorf("合併後的訊息應仍包含 EmptyAfterRetry 市場代碼，實得: %q", reason)
	}
}

type stubPriceClient struct {
	rangeCalls []stubRangeCall
	rangeFn    func(callIndex int, start, end time.Time, marketCode string) ([]moa.TradeRecord, error)

	restDays    []moa.RestDay
	restDaysErr error
}

type stubRangeCall struct {
	start, end time.Time
	marketCode string
}

func (s *stubPriceClient) FetchRange(_ context.Context, start, end time.Time, _, marketCode string) ([]moa.TradeRecord, error) {
	idx := len(s.rangeCalls)
	s.rangeCalls = append(s.rangeCalls, stubRangeCall{start: start, end: end, marketCode: marketCode})
	return s.rangeFn(idx, start, end, marketCode)
}

func (s *stubPriceClient) FetchRestDays(_ context.Context) ([]moa.RestDay, error) {
	return s.restDays, s.restDaysErr
}

type stubExistenceChecker struct {
	hasObservation bool
	err            error
	calls          []string
	beforeDates    []time.Time
}

func (s *stubExistenceChecker) MarketHasAnyObservation(_ context.Context, arg gen.MarketHasAnyObservationParams) (bool, error) {
	s.calls = append(s.calls, arg.MarketCode)
	s.beforeDates = append(s.beforeDates, arg.BeforeDate.Time)
	return s.hasObservation, s.err
}

func TestFetchAllWithEmptyRetrySucceedsAfterRetry(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	stub := &stubPriceClient{
		rangeFn: func(callIndex int, _, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {
			if callIndex == 0 {
				return nil, nil
			}
			return []moa.TradeRecord{{TransDate: "115.07.01", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10}}, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, "800")
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(stillEmpty) != 0 {
		t.Errorf("重試後成功拿到資料，stillEmpty 應為空，實得: %+v", stillEmpty)
	}
	if len(records) != 1 {
		t.Errorf("預期重試拿到 1 筆資料，實得 %d 筆", len(records))
	}
	if len(stub.rangeCalls) != 2 {
		t.Errorf("預期 FetchRange 呼叫 2 次（第一次空、重試 1 次成功），實得 %d 次", len(stub.rangeCalls))
	}
	if len(checker.calls) != 1 || checker.calls[0] != "800" {
		t.Errorf("預期查過市場 800 是否有過去資料一次，實得 %+v", checker.calls)
	}

	if len(checker.beforeDates) != 1 || !checker.beforeDates[0].Equal(end) {
		t.Errorf("預期 MarketHasAnyObservation 的 beforeDate = %v，實得 %+v", end, checker.beforeDates)
	}
}

func TestFetchAllWithEmptyRetryFlagsAfterExhaustingRetries(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			return nil, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, "800")
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 不應回傳錯誤（持續空手不是 infra 錯誤）: %v", err)
	}
	if len(stillEmpty) != 1 {
		t.Fatalf("重試耗盡仍空手，預期 1 個缺口，實得 %d 個: %+v", len(stillEmpty), stillEmpty)
	}
	if !stillEmpty[0].start.Equal(start) || !stillEmpty[0].end.Equal(end) {
		t.Errorf("缺口範圍應是整段查詢範圍 %v~%v，實得 %+v", start, end, stillEmpty[0])
	}
	if len(records) != 0 {
		t.Errorf("持續空手，records 應為空，實得 %d 筆", len(records))
	}
	wantCalls := 1 + emptyResponseMaxRetries
	if len(stub.rangeCalls) != wantCalls {
		t.Errorf("預期 FetchRange 呼叫 %d 次（首次 + %d 次重試），實得 %d 次",
			wantCalls, emptyResponseMaxRetries, len(stub.rangeCalls))
	}
}

func TestFetchAllWithEmptyRetrySkipsRetryWhenNoPriorData(t *testing.T) {
	start := time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2012, 1, 31, 0, 0, 0, 0, time.UTC)
	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			return nil, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: false}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, "999")
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(stillEmpty) != 0 {
		t.Errorf("市場過去沒有任何資料時不該標記缺口，實得: %+v", stillEmpty)
	}
	if len(records) != 0 {
		t.Errorf("預期空結果，實得 %d 筆", len(records))
	}
	if len(stub.rangeCalls) != 1 {
		t.Errorf("市場沒有過去資料時不該重試，預期 FetchRange 只呼叫 1 次，實得 %d 次", len(stub.rangeCalls))
	}
}

func TestFetchAllWithEmptyRetryDegradesGracefullyOnCheckerError(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			return nil, nil
		},
	}
	checker := &stubExistenceChecker{err: errors.New("db 短暫斷線")}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	_, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, "800")
	if err != nil {
		t.Fatalf("checker 失敗不應讓 fetchAllWithEmptyRetry 回傳錯誤: %v", err)
	}
	if len(stillEmpty) != 0 {
		t.Errorf("checker 失敗時不該標記缺口（無法確認是否可疑），實得: %+v", stillEmpty)
	}
	if len(stub.rangeCalls) != 1 {
		t.Errorf("checker 失敗時不該重試，預期 FetchRange 只呼叫 1 次，實得 %d 次", len(stub.rangeCalls))
	}
}

func TestFetchAllWithEmptyRetryRetriesEachEmptyLeafIndependently(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	const marketCode = "800"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, callStart, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			switch {
			case callIndex == 0:
				return nil, moa.ErrResponseTruncated
			case callStart.Equal(start):
				return nil, nil
			default:
				return []moa.TradeRecord{
					{TransDate: moa.ToROCDate(callStart), CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				}, nil
			}
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(stillEmpty) != 1 {
		t.Fatalf("其中一個葉節點重試後仍空手，預期 1 個缺口，即使合併結果非空，實得 %d 個: %+v", len(stillEmpty), stillEmpty)
	}
	if len(records) != 1 {
		t.Errorf("預期合併結果含另一葉節點的 1 筆資料，實得 %d 筆", len(records))
	}
}

func TestFetchAllWithEmptyRetryDetectsDatesMaskedByOlderData(t *testing.T) {
	start := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, _, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {
			if callIndex == 0 {

				return []moa.TradeRecord{
					{TransDate: "115.08.12", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
					{TransDate: "115.08.14", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				}, nil
			}

			return nil, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("預期保留 8/12、8/14 的 2 筆真實資料，實得 %d 筆", len(records))
	}
	if len(stillEmpty) != 1 {
		t.Fatalf("預期偵測到 8/13 這天的缺口並重試後仍空手，實得 %d 個缺口: %+v", len(stillEmpty), stillEmpty)
	}
	want813 := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	if !stillEmpty[0].start.Equal(want813) || !stillEmpty[0].end.Equal(want813) {
		t.Errorf("缺口應精確指向 8/13 這一天，實得 %+v", stillEmpty[0])
	}
	if len(stub.rangeCalls) != 1+emptyResponseMaxRetries {
		t.Errorf("預期首次查詢 1 次 + 對缺口日重試 %d 次，實得 %d 次",
			emptyResponseMaxRetries, len(stub.rangeCalls))
	}
}

func TestFetchAllWithEmptyRetrySkipsMostRecentDayUnderStableLag(t *testing.T) {
	start := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {

			return []moa.TradeRecord{
				{TransDate: "115.08.12", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				{TransDate: "115.08.13", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
			}, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(stillEmpty) != 0 {
		t.Errorf("end 當天（今天）尚未發布資料屬正常延遲，不該被標記為缺口，實得: %+v", stillEmpty)
	}
	if len(records) != 2 {
		t.Errorf("預期保留 8/12、8/13 的 2 筆真實資料，實得 %d 筆", len(records))
	}
	if len(stub.rangeCalls) != 1 {
		t.Errorf("end 當天不該觸發任何重試（也不該觸發 MarketHasAnyObservation 查詢），預期 FetchRange 只呼叫 1 次，實得 %d 次", len(stub.rangeCalls))
	}
	if len(checker.calls) != 0 {
		t.Errorf("沒有任何缺口需要判斷時，不該呼叫 MarketHasAnyObservation，實得 %+v", checker.calls)
	}
}

func TestFetchAllWithEmptyRetryMergesConsecutiveMissingDatesIntoOneLeaf(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, _, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {
			if callIndex == 0 {

				return []moa.TradeRecord{
					{TransDate: "115.08.10", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
					{TransDate: "115.08.13", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				}, nil
			}

			return nil, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("預期保留 8/10、8/13 的 2 筆真實資料，實得 %d 筆", len(records))
	}
	if len(stillEmpty) != 1 {
		t.Fatalf("連續兩天的缺口應合併成 1 個葉節點，實得 %d 個: %+v", len(stillEmpty), stillEmpty)
	}
	want11 := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	want12 := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	if !stillEmpty[0].start.Equal(want11) || !stillEmpty[0].end.Equal(want12) {
		t.Errorf("合併後的葉節點應橫跨 8/11~8/12，實得 %+v", stillEmpty[0])
	}
	wantCalls := 1 + emptyResponseMaxRetries
	if len(stub.rangeCalls) != wantCalls {
		t.Errorf("連續缺口合併後應只有一輪重試（%d 次 HTTP call），不是每天各一輪，預期共 %d 次呼叫，實得 %d 次",
			emptyResponseMaxRetries, wantCalls, len(stub.rangeCalls))
	}
}

func TestFetchAllWithEmptyRetryKeepsNonConsecutiveMissingDatesAsSeparateLeaves(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, _, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {
			if callIndex == 0 {

				return []moa.TradeRecord{
					{TransDate: "115.08.11", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
					{TransDate: "115.08.13", CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				}, nil
			}
			return nil, nil
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	_, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAllWithEmptyRetry 失敗: %v", err)
	}
	if len(stillEmpty) != 2 {
		t.Fatalf("不連續的缺口應保持兩個獨立葉節點，不能誤合併，實得 %d 個: %+v", len(stillEmpty), stillEmpty)
	}
	want10 := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	want12 := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	if !stillEmpty[0].start.Equal(want10) || !stillEmpty[0].end.Equal(want10) {
		t.Errorf("第一個葉節點應精確指向 8/10（單日，不與 8/12 合併），實得 %+v", stillEmpty[0])
	}
	if !stillEmpty[1].start.Equal(want12) || !stillEmpty[1].end.Equal(want12) {
		t.Errorf("第二個葉節點應精確指向 8/12（單日，不與 8/10 合併），實得 %+v", stillEmpty[1])
	}
	wantCalls := 1 + 2*emptyResponseMaxRetries
	if len(stub.rangeCalls) != wantCalls {
		t.Errorf("兩個獨立葉節點應各自重試 %d 次，預期共 %d 次呼叫，實得 %d 次",
			emptyResponseMaxRetries, wantCalls, len(stub.rangeCalls))
	}
}

func TestRetryEmptyLeafHandlesTruncationOnRecovery(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, callStart, callEnd time.Time, _ string) ([]moa.TradeRecord, error) {
			switch callIndex {
			case 0:

				return nil, nil
			case 1:

				return nil, moa.ErrResponseTruncated
			default:

				return []moa.TradeRecord{
					{TransDate: moa.ToROCDate(callStart), CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
				}, nil
			}
		},
	}
	checker := &stubExistenceChecker{hasObservation: true}
	u := &PriceIngest{client: stub, retryDelay: time.Millisecond}

	records, stillEmpty, err := u.fetchAllWithEmptyRetry(context.Background(), checker, start, end, marketCode)
	if err != nil {
		t.Fatalf("重試期間 MOA 恢復且觸頂，應該正確切分拿到資料，不應回傳錯誤: %v", err)
	}
	if len(stillEmpty) != 0 {
		t.Errorf("重試成功拿到資料，stillEmpty 應為空，實得: %+v", stillEmpty)
	}
	if len(records) != 2 {
		t.Errorf("預期切分後兩個單日各回一筆、共 2 筆，實得 %d 筆", len(records))
	}
}

func TestHandleSplitsRangeOnTruncation(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	const marketCode = "104"

	stub := &stubPriceClient{
		rangeFn: func(callIndex int, start, end time.Time, _ string) ([]moa.TradeRecord, error) {
			if callIndex == 0 {

				return nil, moa.ErrResponseTruncated
			}

			return []moa.TradeRecord{
				{TransDate: moa.ToROCDate(start), CropCode: "LA1", MarketCode: marketCode, AvgPrice: 10},
			}, nil
		},
	}

	u := &PriceIngest{client: stub}
	records, _, err := u.fetchAll(context.Background(), start, end, marketCode)
	if err != nil {
		t.Fatalf("fetchAll 失敗: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("預期切分後兩個單日各回一筆、共 2 筆，實得 %d 筆", len(records))
	}
	if len(stub.rangeCalls) != 3 {
		t.Fatalf("預期呼叫 3 次（完整區間 1 次 + 切分後 2 次），實得 %d 次: %+v", len(stub.rangeCalls), stub.rangeCalls)
	}
	if !stub.rangeCalls[0].start.Equal(start) || !stub.rangeCalls[0].end.Equal(end) {
		t.Errorf("第一次呼叫應是完整區間 [%v, %v]，實得 [%v, %v]",
			start, end, stub.rangeCalls[0].start, stub.rangeCalls[0].end)
	}
	if !stub.rangeCalls[1].start.Equal(start) || !stub.rangeCalls[1].end.Equal(start) {
		t.Errorf("第二次呼叫應是切分後的前半日 [%v, %v]，實得 [%v, %v]",
			start, start, stub.rangeCalls[1].start, stub.rangeCalls[1].end)
	}
	if !stub.rangeCalls[2].start.Equal(end) || !stub.rangeCalls[2].end.Equal(end) {
		t.Errorf("第三次呼叫應是切分後的後半日 [%v, %v]，實得 [%v, %v]",
			end, end, stub.rangeCalls[2].start, stub.rangeCalls[2].end)
	}
	for i, c := range stub.rangeCalls {
		if c.marketCode != marketCode {
			t.Errorf("第 %d 次呼叫的 marketCode = %q，預期切分不改變市場過濾參數 %q", i, c.marketCode, marketCode)
		}
	}
}

func TestHandleSplitSingleDayTruncationReturnsError(t *testing.T) {
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			return nil, moa.ErrResponseTruncated
		},
	}
	u := &PriceIngest{client: stub}
	_, _, err := u.fetchAll(context.Background(), day, day, "104")
	if err == nil {
		t.Fatal("單日仍觸頂應回傳錯誤，不能靜默丟資料")
	}
	if len(stub.rangeCalls) != 1 {
		t.Errorf("單日觸頂不該遞迴呼叫，預期 1 次，實得 %d 次", len(stub.rangeCalls))
	}
}

func TestHandleSplitPropagatesNonTruncationError(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	wantErr := context.DeadlineExceeded
	stub := &stubPriceClient{
		rangeFn: func(_ int, _, _ time.Time, _ string) ([]moa.TradeRecord, error) {
			return nil, wantErr
		},
	}
	u := &PriceIngest{client: stub}
	_, _, err := u.fetchAll(context.Background(), start, end, "104")
	if !errors.Is(err, wantErr) {
		t.Errorf("非截斷錯誤應原樣回傳，實得 %v", err)
	}
	if len(stub.rangeCalls) != 1 {
		t.Errorf("非截斷錯誤不該觸發切分重試，預期 1 次呼叫，實得 %d 次", len(stub.rangeCalls))
	}
}

func TestListMarketCodesDedupesByFirstAppearance(t *testing.T) {
	stub := &stubPriceClient{
		restDays: []moa.RestDay{
			{MarketCode: "104", MarketName: "台北二", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)},
			{MarketCode: "108", MarketName: "台中", Date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)},
			{MarketCode: "104", MarketName: "台北二", Date: time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)},
		},
	}
	u := &PriceIngest{client: stub}
	codes, err := u.listMarketCodes(context.Background())
	if err != nil {
		t.Fatalf("listMarketCodes 失敗: %v", err)
	}
	want := []string{"104", "108"}
	if len(codes) != len(want) {
		t.Fatalf("預期 %d 個 distinct market code，實得 %d 個: %v", len(want), len(codes), codes)
	}
	for i, w := range want {
		if codes[i] != w {
			t.Errorf("codes[%d] = %q，預期 %q（順序應是首次出現順序）", i, codes[i], w)
		}
	}
}

func TestHandleSumsFetchedAcrossDistinctMarketsWithoutDoubleCounting(t *testing.T) {
	stub := &stubPriceClient{
		restDays: []moa.RestDay{
			{MarketCode: "104", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)},
			{MarketCode: "104", Date: time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)},
			{MarketCode: "108", Date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)},
		},
		rangeFn: func(_ int, callStart, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {

			return []moa.TradeRecord{
				{TransDate: moa.ToROCDate(callStart), CropCode: "-", CropName: "休市", MarketCode: marketCode, AvgPrice: 0},
			}, nil
		},
	}

	u := &PriceIngest{client: stub}
	start := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := u.Handle(context.Background(), start, start)
	if err != nil {
		t.Fatalf("Handle 失敗: %v", err)
	}
	if stats.Fetched != 2 {
		t.Errorf("預期 Fetched=2（104 與 108 各 1 筆，104 不因重複出現在休市日曆而被算兩次），實得 %d", stats.Fetched)
	}
	if stats.Inserted != 0 || stats.Revised != 0 {
		t.Errorf("全部是休市哨兵列，Inserted/Revised 應皆為 0，實得 Inserted=%d Revised=%d",
			stats.Inserted, stats.Revised)
	}

	if len(stub.rangeCalls) != 2 {
		t.Fatalf("預期 FetchRange 只被呼叫 2 次（每個 distinct 市場各 1 次），實得 %d 次: %+v",
			len(stub.rangeCalls), stub.rangeCalls)
	}
	gotMarkets := make(map[string]bool, len(stub.rangeCalls))
	for _, c := range stub.rangeCalls {
		gotMarkets[c.marketCode] = true
	}
	if !gotMarkets["104"] || !gotMarkets["108"] {
		t.Errorf("預期查詢過市場 104 與 108，實際呼叫: %+v", stub.rangeCalls)
	}
}

func TestHandleSkipsRecordWithUnparseableTradeDateAndContinues(t *testing.T) {
	stub := &stubPriceClient{
		restDays: []moa.RestDay{
			{MarketCode: "104", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)},
			{MarketCode: "108", Date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)},
		},
		rangeFn: func(_ int, callStart, _ time.Time, marketCode string) ([]moa.TradeRecord, error) {
			switch marketCode {
			case "104":
				return []moa.TradeRecord{

					{TransDate: "not-a-date", CropCode: "LA1", CropName: "甘藍-初秋",
						MarketCode: "104", MarketName: "台北二", AvgPrice: 46.2},

					{TransDate: moa.ToROCDate(callStart), CropCode: "-", CropName: "休市", MarketCode: "104", AvgPrice: 0},
				}, nil
			case "108":
				return []moa.TradeRecord{
					{TransDate: moa.ToROCDate(callStart), CropCode: "-", CropName: "休市", MarketCode: "108", AvgPrice: 0},
				}, nil
			default:
				t.Fatalf("非預期的 marketCode: %q", marketCode)
				return nil, nil
			}
		},
	}

	u := &PriceIngest{client: stub}
	start := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := u.Handle(context.Background(), start, start)
	if err != nil {
		t.Fatalf("Handle 不應因單筆壞資料而回傳錯誤，實得: %v", err)
	}
	if stats.Skipped != 1 {
		t.Errorf("Skipped = %d，預期 1（僅市場 104 那筆壞 TransDate）", stats.Skipped)
	}
	if stats.Fetched != 3 {
		t.Errorf("Fetched = %d，預期 3（市場 104 兩筆 + 市場 108 一筆，過濾前）", stats.Fetched)
	}
	if stats.Inserted != 0 || stats.Revised != 0 {
		t.Errorf("除了壞資料外其餘都是休市哨兵，Inserted/Revised 應皆為 0，實得 Inserted=%d Revised=%d",
			stats.Inserted, stats.Revised)
	}
	if len(stub.rangeCalls) != 2 {
		t.Fatalf("預期市場 104 與 108 都被查詢到（各 1 次），實得 %d 次: %+v",
			len(stub.rangeCalls), stub.rangeCalls)
	}
}

func TestHandleRejectsReversedRange(t *testing.T) {
	u := &PriceIngest{client: &stubPriceClient{}}
	start := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, -1)
	stats, err := u.Handle(context.Background(), start, end)
	if err == nil {
		t.Fatal("end 早於 start 應回傳錯誤")
	}
	if stats.Fetched != 0 || stats.Inserted != 0 || stats.Revised != 0 {
		t.Errorf("反轉區間應立即回傳零值統計，實得 %+v", stats)
	}
}

func TestHandleRejectsEmptyMarketList(t *testing.T) {
	stub := &stubPriceClient{restDays: nil, restDaysErr: nil}
	u := &PriceIngest{client: stub}
	start := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := u.Handle(context.Background(), start, start)
	if err == nil {
		t.Fatal("休市日曆回傳空市場清單時應回傳錯誤，不能靜默當成 Fetched=0 的成功執行")
	}
	if stats.Fetched != 0 || stats.Inserted != 0 || stats.Revised != 0 {
		t.Errorf("空市場清單應立即回傳零值統計，實得 %+v", stats)
	}
	if len(stub.rangeCalls) != 0 {
		t.Errorf("空市場清單時不該有任何 FetchRange 呼叫，實得 %d 次", len(stub.rangeCalls))
	}
}

func TestMissingCalendarDatesFindsDatesWithNoRecords(t *testing.T) {
	start := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	records := []moa.TradeRecord{
		{TransDate: "115.08.13", CropCode: "LA1", MarketCode: "104", AvgPrice: 10},
	}

	missing := missingCalendarDates(records, start, end)

	want := []time.Time{
		time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
	if len(missing) != len(want) {
		t.Fatalf("預期 %d 個缺口日期，實得 %d 個: %v", len(want), len(missing), missing)
	}
	for i, d := range want {
		if !missing[i].Equal(d) {
			t.Errorf("missing[%d] = %v，預期 %v", i, missing[i], d)
		}
	}
}

func TestMissingCalendarDatesTreatsRestDaySentinelAsPresent(t *testing.T) {
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	records := []moa.TradeRecord{
		{TransDate: "115.08.17", CropCode: "999", CropName: "休市", MarketCode: "104"},
	}

	missing := missingCalendarDates(records, start, end)

	if len(missing) != 0 {
		t.Errorf("休市哨兵列本身就代表這天有交代，不該被列為缺口，實得: %v", missing)
	}
}

func TestMissingCalendarDatesReturnsNilWhenAllDatesPresent(t *testing.T) {
	start := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	records := []moa.TradeRecord{
		{TransDate: "115.08.13", CropCode: "LA1", MarketCode: "104", AvgPrice: 10},
	}

	if missing := missingCalendarDates(records, start, end); len(missing) != 0 {
		t.Errorf("所有日期都有記錄時應回傳空，實得: %v", missing)
	}
}

func TestMergeConsecutiveDatesMergesRunsAndReturnsNilForEmptyInput(t *testing.T) {
	if got := mergeConsecutiveDates(nil); got != nil {
		t.Errorf("空輸入應回傳 nil，實得: %+v", got)
	}

	d := func(day int) time.Time { return time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC) }

	got := mergeConsecutiveDates([]time.Time{d(11), d(12), d(13)})
	want := []emptyLeaf{{start: d(11), end: d(13)}}
	if len(got) != len(want) || got[0].start != want[0].start || got[0].end != want[0].end {
		t.Errorf("連續三天應合併成單一橫跨區間的葉節點，預期 %+v，實得 %+v", want, got)
	}

	got = mergeConsecutiveDates([]time.Time{d(10), d(12), d(14)})
	want = []emptyLeaf{{start: d(10), end: d(10)}, {start: d(12), end: d(12)}, {start: d(14), end: d(14)}}
	if len(got) != len(want) {
		t.Fatalf("三個互不相鄰的日期應各自保持獨立葉節點，預期 %d 個，實得 %d 個: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i].start != want[i].start || got[i].end != want[i].end {
			t.Errorf("第 %d 個葉節點應為 %+v，實得 %+v", i, want[i], got[i])
		}
	}

	got = mergeConsecutiveDates([]time.Time{d(11), d(12), d(14)})
	want = []emptyLeaf{{start: d(11), end: d(12)}, {start: d(14), end: d(14)}}
	if len(got) != len(want) {
		t.Fatalf("連續段與獨立日混合時應各自正確分組，預期 %d 個，實得 %d 個: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i].start != want[i].start || got[i].end != want[i].end {
			t.Errorf("第 %d 個葉節點應為 %+v，實得 %+v", i, want[i], got[i])
		}
	}
}

func TestAttentionReasonFiresOnNewlyRejectedIdentities(t *testing.T) {
	s := IngestStats{
		Fetched: 1000, Inserted: 993,
		RejectedDefects: map[moa.RejectReason]int{moa.RejectMissingCropName: 7},
		NewRejectedIdentities: []RejectedIdentity{
			{CropCode: "FE800", MarketCode: "105", Reason: moa.RejectMissingCropName},
			{CropCode: "FH010", MarketCode: "105", Reason: moa.RejectMissingCropName},
		},
	}
	reason := s.AttentionReason()
	if reason == "" {
		t.Fatal("新出現被隔離的身分時 AttentionReason 應回傳非空字串")
	}
	for _, want := range []string{"missing_crop_name", "FE800", "FH010"} {
		if !strings.Contains(reason, want) {
			t.Errorf("AttentionReason 應點名 %q，實得: %q", want, reason)
		}
	}
}

func TestAttentionReasonSilentWhenRejectedIdentitiesAreAllKnown(t *testing.T) {
	s := IngestStats{
		Fetched: 1000, Inserted: 993,
		RejectedDefects: map[moa.RejectReason]int{moa.RejectMissingCropName: 7},
	}
	if reason := s.AttentionReason(); reason != "" {
		t.Errorf("已知身分持續被隔離不得告警，實得: %q", reason)
	}
}

func TestAttentionReasonDoesNotConflateRejectionsWithParseFailures(t *testing.T) {
	s := IngestStats{
		Fetched: 7, Skipped: 0,
		RejectedDefects: map[moa.RejectReason]int{moa.RejectMissingCropName: 7},
	}
	if reason := s.AttentionReason(); reason != "" {
		t.Errorf("隔離不等於解析失敗，不得觸發 Skipped 判準，實得: %q", reason)
	}
}
