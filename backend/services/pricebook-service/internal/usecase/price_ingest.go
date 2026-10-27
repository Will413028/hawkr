package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const sourceMOA = "moa"

const (
	emptyResponseMaxRetries = 2
	emptyResponseRetryDelay = 5 * time.Second
)

const ingestBatchSize = 5000

func RecordDigest(r moa.TradeRecord) string {
	payload := fmt.Sprintf("%s|%s|%s|%.4f|%.4f|%.4f|%.4f|%.3f",
		r.TransDate, r.CropCode, r.MarketCode,
		r.AvgPrice, r.UpperPrice, r.MiddlePrice, r.LowerPrice, r.TransQuantity)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

type EmptyLeafGap struct {
	MarketCode string
	Start, End time.Time
}

type RejectedIdentity struct {
	CropCode   string
	MarketCode string
	Reason     moa.RejectReason
}

const (
	sourceMOATrades   = "moa_agri_products_trans_type"
	sourceMOARestDays = "moa_market_rest_day_farm_wcf"
)

type rejectedRegister struct {
	source string
	seen   map[RejectedIdentity]*rejectedRun
}

type rejectedRun struct {
	count               int64
	firstDate, lastDate string
}

func newRejectedRegister(source string) *rejectedRegister {
	return &rejectedRegister{source: source, seen: make(map[RejectedIdentity]*rejectedRun)}
}

func (g *rejectedRegister) add(cropCode, marketCode, tradeDate string, reason moa.RejectReason) {
	id := RejectedIdentity{
		CropCode:   strings.TrimSpace(cropCode),
		MarketCode: strings.TrimSpace(marketCode),
		Reason:     reason,
	}
	date := strings.TrimSpace(tradeDate)
	run, ok := g.seen[id]
	if !ok {
		g.seen[id] = &rejectedRun{count: 1, firstDate: date, lastDate: date}
		return
	}
	run.count++

	if date < run.firstDate {
		run.firstDate = date
	}
	if date > run.lastDate {
		run.lastDate = date
	}
}

func (g *rejectedRegister) flush(ctx context.Context, q *gen.Queries, stats *IngestStats) error {
	if len(g.seen) == 0 {
		return nil
	}
	ids := make([]RejectedIdentity, 0, len(g.seen))
	for id := range g.seen {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		if ids[i].CropCode != ids[j].CropCode {
			return ids[i].CropCode < ids[j].CropCode
		}
		if ids[i].MarketCode != ids[j].MarketCode {
			return ids[i].MarketCode < ids[j].MarketCode
		}
		return ids[i].Reason < ids[j].Reason
	})

	seenAt := pgxutil.TimestamptzParam(time.Now().UTC())
	if stats.RejectedDefects == nil {
		stats.RejectedDefects = make(map[moa.RejectReason]int, len(ids))
	}
	for _, id := range ids {
		run := g.seen[id]
		stats.RejectedDefects[id.Reason] += int(run.count)
		isNew, err := q.RecordRejectedIdentity(ctx, gen.RecordRejectedIdentityParams{
			Source:         g.source,
			CropCode:       id.CropCode,
			MarketCode:     id.MarketCode,
			Reason:         string(id.Reason),
			Occurrences:    run.count,
			SeenAt:         seenAt,
			FirstTradeDate: run.firstDate,
			LastTradeDate:  run.lastDate,
		})
		if err != nil {
			return fmt.Errorf("登記隔離身分失敗（crop=%s, market=%s, reason=%s）: %w",
				id.CropCode, id.MarketCode, id.Reason, err)
		}
		if isNew {
			stats.NewRejectedIdentities = append(stats.NewRejectedIdentities, id)
		}
	}
	zap.L().Warn("price_ingest: 上游列因身分不完整被隔離",
		zap.Int("identities", len(ids)),
		zap.Int("new_identities", len(stats.NewRejectedIdentities)))
	return nil
}

type IngestStats struct {
	Fetched  int
	Inserted int
	Revised  int
	Skipped  int

	EmptyAfterRetry []EmptyLeafGap

	PartitionsAttempted int

	RejectedDefects map[moa.RejectReason]int

	NewRejectedIdentities []RejectedIdentity
}

func (s IngestStats) AttentionReason() string {
	var reasons []string

	if s.Skipped > 0 && s.Inserted+s.Revised == 0 {
		reasons = append(reasons, fmt.Sprintf(
			"抓到 %d 列但一列都沒寫入（skipped=%d，inserted+revised=0）：疑似上游回應格式變更，需人工確認",
			s.Fetched, s.Skipped))
	}

	if s.PartitionsAttempted > 0 && s.Fetched == 0 {
		reasons = append(reasons, fmt.Sprintf(
			"向上游求取 %d 個分區，一列都沒拿到：疑似上游整體無回應或尚未上架，需人工確認",
			s.PartitionsAttempted))
	}
	if len(s.EmptyAfterRetry) > 0 {

		byMarket := make(map[string][]EmptyLeafGap, len(s.EmptyAfterRetry))
		for _, g := range s.EmptyAfterRetry {
			byMarket[g.MarketCode] = append(byMarket[g.MarketCode], g)
		}
		codes := make([]string, 0, len(byMarket))
		for code := range byMarket {
			codes = append(codes, code)
		}
		sort.Strings(codes)

		parts := make([]string, 0, len(codes))
		for _, code := range codes {
			seenRanges := make(map[string]struct{})
			var ranges []string
			for _, g := range byMarket[code] {
				r := formatGapRange(g.Start, g.End)
				if _, ok := seenRanges[r]; ok {
					continue
				}
				seenRanges[r] = struct{}{}
				ranges = append(ranges, r)
			}
			parts = append(parts, fmt.Sprintf("%s[%s]", code, strings.Join(ranges, ",")))
		}
		reasons = append(reasons, fmt.Sprintf(
			"%d 個市場過去曾有資料，但這次重試 %d 次後仍空手（%s）：疑似 MOA 壅塞造成靜默資料遺失，需人工確認",
			len(codes), emptyResponseMaxRetries, strings.Join(parts, "; ")))
	}

	if len(s.NewRejectedIdentities) > 0 {
		byReason := make(map[moa.RejectReason][]string, len(s.NewRejectedIdentities))
		for _, id := range s.NewRejectedIdentities {
			byReason[id.Reason] = append(byReason[id.Reason], id.CropCode)
		}
		causes := make([]string, 0, len(byReason))
		for reason := range byReason {
			causes = append(causes, string(reason))
		}
		sort.Strings(causes)

		parts := make([]string, 0, len(causes))
		for _, cause := range causes {
			codes := byReason[moa.RejectReason(cause)]
			sort.Strings(codes)
			parts = append(parts, fmt.Sprintf("%s: %s",
				cause, strings.Join(slices.Compact(codes), "、")))
		}
		reasons = append(reasons, fmt.Sprintf(
			"上游新增 %d 個無法使用的身分並已隔離（%s）：需人工確認是否要補來源或接受缺漏",
			len(s.NewRejectedIdentities), strings.Join(parts, "; ")))
	}

	return strings.Join(reasons, "; ")
}

func formatGapRange(start, end time.Time) string {
	if start.Equal(end) {
		return start.Format("2006-01-02")
	}
	return fmt.Sprintf("%s~%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

var errParseTradeDate = errors.New("解析交易日期失敗")

type priceClient interface {
	FetchRange(ctx context.Context, start, end time.Time, cropCode, marketCode string) ([]moa.TradeRecord, error)
	FetchRestDays(ctx context.Context) ([]moa.RestDay, error)
}

type marketExistenceChecker interface {
	MarketHasAnyObservation(ctx context.Context, arg gen.MarketHasAnyObservationParams) (bool, error)
}

type PriceIngest struct {
	client priceClient
	pool   *pgxpool.Pool
	tsid   func() int64

	retryDelay time.Duration

	batchSize int
}

func NewPriceIngest(c *moa.Client, pool *pgxpool.Pool, tsid func() int64) *PriceIngest {
	return &PriceIngest{client: c, pool: pool, tsid: tsid, retryDelay: emptyResponseRetryDelay, batchSize: ingestBatchSize}
}

func NewPriceIngestWithClient(c priceClient, pool *pgxpool.Pool, tsid func() int64, retryDelay time.Duration) *PriceIngest {
	return &PriceIngest{client: c, pool: pool, tsid: tsid, retryDelay: retryDelay, batchSize: ingestBatchSize}
}

func (u *PriceIngest) SetRetryDelayForTest(d time.Duration) {
	u.retryDelay = d
}

func (u *PriceIngest) SetBatchSizeForTest(n int) {
	u.batchSize = n
}

func (u *PriceIngest) Handle(ctx context.Context, start, end time.Time) (IngestStats, error) {
	var stats IngestStats

	if end.Before(start) {
		return stats, fmt.Errorf("行情 ingest 區間反轉：start=%s 晚於 end=%s",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	marketCodes, err := u.listMarketCodes(ctx)
	if err != nil {
		return stats, fmt.Errorf("取得市場清單失敗: %w", err)
	}

	if len(marketCodes) == 0 {
		return stats, fmt.Errorf("休市日曆未回傳任何市場，無法列舉行情查詢對象")
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))
	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())

	cache := &upsertCache{
		groupIDs:  make(map[string]int64),
		cropIDs:   make(map[string]int64),
		marketIDs: make(map[string]int64),
	}

	rejected := newRejectedRegister(sourceMOATrades)

	batchSize := u.batchSize
	if batchSize <= 0 {
		batchSize = ingestBatchSize
	}

	completed := newPartitionOutcome()

	defer func() {
		if err := u.persistPartitionOutcome(ctx, q, completed); err != nil {

			zap.L().Warn("price_ingest: 標記分區完成失敗（資料已寫入，分區狀態可能落後）",
				zap.Error(err))
		}
	}()

	for _, marketCode := range marketCodes {
		records, stillEmptyLeaves, err := u.fetchAllWithEmptyRetry(ctx, q, start, end, marketCode)
		if err != nil {
			return stats, fmt.Errorf("抓取行情失敗（market=%s）: %w", marketCode, err)
		}
		for _, leaf := range stillEmptyLeaves {
			stats.EmptyAfterRetry = append(stats.EmptyAfterRetry, EmptyLeafGap{
				MarketCode: marketCode, Start: leaf.start, End: leaf.end,
			})
		}
		stats.Fetched += len(records)

		var valid []moa.ValidTradeRecord
		for _, r := range records {
			v, reason := r.Validate()
			if reason == "" {
				valid = append(valid, v)
				continue
			}

			if reason.IsDataDefect() {
				rejected.add(r.CropCode, r.MarketCode, r.TransDate, reason)
			}
		}

		for i := 0; i < len(valid); i += batchSize {
			end := i + batchSize
			if end > len(valid) {
				end = len(valid)
			}
			inserted, revised, skipped, err := u.ingestBatch(ctx, q, cache, valid[i:end], availableAt)
			if err != nil {
				return stats, fmt.Errorf("批次寫入行情失敗（market=%s）: %w", marketCode, err)
			}
			stats.Inserted += inserted
			stats.Revised += revised
			stats.Skipped += skipped
		}

		for _, v := range valid {
			r := v.Record()
			day, parseErr := moa.ParseROCDate(r.TransDate)
			if parseErr != nil {
				continue
			}
			if marketID, ok := cache.marketIDs[r.MarketCode]; ok {
				completed.markComplete(partitionKey{marketID: marketID, date: day})
			}
		}
	}

	if err := rejected.flush(ctx, q, &stats); err != nil {
		return stats, err
	}

	if stats.Fetched == 0 {
		zap.L().Warn("price_ingest: 本次執行跨所有市場的 Fetched 總數為 0，可能是 MOA 端異常",
			zap.String("start", start.Format("2006-01-02")),
			zap.String("end", end.Format("2006-01-02")),
			zap.Int("market_count", len(marketCodes)))
	}

	return stats, nil
}

func (u *PriceIngest) listMarketCodes(ctx context.Context) ([]string, error) {
	days, err := u.client.FetchRestDays(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(days))
	codes := make([]string, 0, len(days))
	for _, d := range days {
		if _, ok := seen[d.MarketCode]; ok {
			continue
		}
		seen[d.MarketCode] = struct{}{}
		codes = append(codes, d.MarketCode)
	}
	return codes, nil
}

type upsertCache struct {
	groupIDs  map[string]int64
	cropIDs   map[string]int64
	marketIDs map[string]int64
}

type resolvedRecord struct {
	marketID    int64
	cropID      int64
	tradeDate   time.Time
	digest      string
	avgPrice    pgtype.Numeric
	upperPrice  pgtype.Numeric
	middlePrice pgtype.Numeric
	lowerPrice  pgtype.Numeric
	volume      pgtype.Numeric
}

func (u *PriceIngest) resolveRecord(
	ctx context.Context,
	q *gen.Queries,
	cache *upsertCache,
	v moa.ValidTradeRecord,
) (resolvedRecord, error) {
	var rr resolvedRecord
	r := v.Record()

	tradeDate, err := moa.ParseROCDate(r.TransDate)
	if err != nil {
		return rr, fmt.Errorf("%w（market=%s, crop=%s, TransDate=%q）: %w",
			errParseTradeDate, r.MarketCode, r.CropCode, r.TransDate, err)
	}
	rr.tradeDate = tradeDate

	group, variant := moa.SplitCropName(r.CropName)
	groupCode := moa.GroupCode(group, r.TcType)
	categoryCode := stringOrNil(r.TcType)

	groupID, ok := cache.groupIDs[groupCode]
	if !ok {
		groupID, err = q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
			ID: u.tsid(), GroupCode: groupCode, Name: group, CategoryCode: categoryCode,
		})
		if err != nil {
			return rr, fmt.Errorf("upsert crop_group 失敗（%s）: %w", groupCode, err)
		}
		if err := registerCropGroupForms(ctx, q, groupCode, group); err != nil {
			return rr, err
		}
		cache.groupIDs[groupCode] = groupID
	}

	cropID, ok := cache.cropIDs[r.CropCode]
	if !ok {
		variantName := stringOrNil(variant)
		cropID, err = q.UpsertCrop(ctx, gen.UpsertCropParams{
			ID: u.tsid(), CropCode: r.CropCode, GroupID: groupID, Name: r.CropName,
			VariantName: variantName, CategoryCode: categoryCode,
		})
		if err != nil {
			return rr, fmt.Errorf("upsert crop 失敗（%s）: %w", r.CropCode, err)
		}
		if err := registerCropForms(ctx, q, r.CropCode, r.CropName, variant); err != nil {
			return rr, err
		}
		cache.cropIDs[r.CropCode] = cropID
	}
	rr.cropID = cropID

	marketID, ok := cache.marketIDs[r.MarketCode]
	if !ok {
		marketID, err = q.UpsertMarket(ctx, gen.UpsertMarketParams{
			ID: u.tsid(), MarketCode: r.MarketCode, Name: r.MarketName,
		})
		if err != nil {
			return rr, fmt.Errorf("upsert market 失敗（%s）: %w", r.MarketCode, err)
		}
		cache.marketIDs[r.MarketCode] = marketID
	}
	rr.marketID = marketID

	rr.digest = RecordDigest(r)

	rr.avgPrice, err = postgres.NumericFromString(fmt.Sprintf("%.4f", r.AvgPrice))
	if err != nil {
		return rr, fmt.Errorf("avg_price 轉換失敗: %w", err)
	}
	rr.upperPrice, err = postgres.NumericFromString(fmt.Sprintf("%.4f", r.UpperPrice))
	if err != nil {
		return rr, fmt.Errorf("upper_price 轉換失敗: %w", err)
	}
	rr.middlePrice, err = postgres.NumericFromString(fmt.Sprintf("%.4f", r.MiddlePrice))
	if err != nil {
		return rr, fmt.Errorf("middle_price 轉換失敗: %w", err)
	}
	rr.lowerPrice, err = postgres.NumericFromString(fmt.Sprintf("%.4f", r.LowerPrice))
	if err != nil {
		return rr, fmt.Errorf("lower_price 轉換失敗: %w", err)
	}
	rr.volume, err = postgres.NumericFromString(fmt.Sprintf("%.3f", r.TransQuantity))
	if err != nil {
		return rr, fmt.Errorf("volume_kg 轉換失敗: %w", err)
	}

	return rr, nil
}

type revisionKey struct {
	cropID    int64
	tradeDate time.Time
}

type revisionState struct {
	revisionNo int32
	digest     string
}

func (u *PriceIngest) ingestBatch(
	ctx context.Context,
	q *gen.Queries,
	cache *upsertCache,
	records []moa.ValidTradeRecord,
	availableAt pgtype.Timestamptz,
) (inserted, revised, skipped int, err error) {
	resolved := make([]resolvedRecord, 0, len(records))
	for _, v := range records {
		r := v.Record()
		rr, rerr := u.resolveRecord(ctx, q, cache, v)
		if rerr != nil {
			if errors.Is(rerr, errParseTradeDate) {
				zap.L().Warn("price_ingest: 跳過單筆無法解析的交易日期",
					zap.String("market_code", r.MarketCode),
					zap.String("crop_code", r.CropCode),
					zap.String("trans_date", r.TransDate),
					zap.Error(rerr))
				skipped++
				continue
			}
			return inserted, revised, skipped, rerr
		}
		resolved = append(resolved, rr)
	}
	if len(resolved) == 0 {
		return inserted, revised, skipped, nil
	}

	marketID := resolved[0].marketID
	cropIDs := make([]int64, len(resolved))
	tradeDates := make([]pgtype.Date, len(resolved))
	for i, rr := range resolved {
		cropIDs[i] = rr.cropID
		tradeDates[i] = pgxutil.DateParam(&rr.tradeDate)
	}

	existingRows, err := q.LatestObservationRevisionsByMarket(ctx, gen.LatestObservationRevisionsByMarketParams{
		MarketID:   marketID,
		CropIds:    cropIDs,
		TradeDates: tradeDates,
	})
	if err != nil {
		return inserted, revised, skipped, fmt.Errorf("批次查詢既有 revision 失敗: %w", err)
	}

	known := make(map[revisionKey]revisionState, len(existingRows))
	for _, e := range existingRows {
		known[revisionKey{cropID: e.CropID, tradeDate: e.TradeDate.Time}] =
			revisionState{revisionNo: e.RevisionNo, digest: e.SourceDigest}
	}

	var (
		ids, marketIDs, batchCropIDs []int64
		batchTradeDates              []pgtype.Date
		revisionNos                  []int32
		availableAts                 []pgtype.Timestamptz
		avgPrices, upperPrices       []pgtype.Numeric
		middlePrices, lowerPrices    []pgtype.Numeric
		volumeKgs                    []pgtype.Numeric
		sources, sourceDigests       []string
	)

	for _, rr := range resolved {
		k := revisionKey{cropID: rr.cropID, tradeDate: rr.tradeDate}
		prev, ok := known[k]
		var revisionNo int32
		switch {
		case !ok:
			revisionNo = 1
			inserted++
		case prev.digest == rr.digest:
			continue
		default:
			revisionNo = prev.revisionNo + 1
			revised++
		}
		known[k] = revisionState{revisionNo: revisionNo, digest: rr.digest}

		ids = append(ids, u.tsid())
		marketIDs = append(marketIDs, rr.marketID)
		batchCropIDs = append(batchCropIDs, rr.cropID)
		batchTradeDates = append(batchTradeDates, pgxutil.DateParam(&rr.tradeDate))
		revisionNos = append(revisionNos, revisionNo)
		availableAts = append(availableAts, availableAt)
		avgPrices = append(avgPrices, rr.avgPrice)
		upperPrices = append(upperPrices, rr.upperPrice)
		middlePrices = append(middlePrices, rr.middlePrice)
		lowerPrices = append(lowerPrices, rr.lowerPrice)
		volumeKgs = append(volumeKgs, rr.volume)
		sources = append(sources, sourceMOA)
		sourceDigests = append(sourceDigests, rr.digest)
	}

	if len(ids) == 0 {
		return inserted, revised, skipped, nil
	}

	if err := q.BatchInsertPriceObservation(ctx, gen.BatchInsertPriceObservationParams{
		Ids: ids, MarketIds: marketIDs, CropIds: batchCropIDs, TradeDates: batchTradeDates,
		RevisionNos: revisionNos, AvailableAts: availableAts,
		AvgPrices: avgPrices, UpperPrices: upperPrices, MiddlePrices: middlePrices,
		LowerPrices: lowerPrices, VolumeKgs: volumeKgs,
		Sources: sources, SourceDigests: sourceDigests,
	}); err != nil {
		return inserted, revised, skipped, fmt.Errorf("批次寫入 price_observation 失敗: %w", err)
	}
	return inserted, revised, skipped, nil
}

type emptyLeaf struct {
	start, end time.Time
}

const stableLagDays = 1

func missingCalendarDates(records []moa.TradeRecord, start, end time.Time) []time.Time {
	present := make(map[time.Time]struct{}, len(records))
	for _, r := range records {
		d, err := moa.ParseROCDate(r.TransDate)
		if err != nil {
			continue
		}
		present[d] = struct{}{}
	}

	var missing []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if _, ok := present[d]; !ok {
			missing = append(missing, d)
		}
	}
	return missing
}

func mergeConsecutiveDates(dates []time.Time) []emptyLeaf {
	if len(dates) == 0 {
		return nil
	}
	leaves := make([]emptyLeaf, 0, len(dates))
	leafStart := dates[0]
	leafEnd := dates[0]
	for _, d := range dates[1:] {
		if d.Equal(leafEnd.AddDate(0, 0, 1)) {
			leafEnd = d
			continue
		}
		leaves = append(leaves, emptyLeaf{start: leafStart, end: leafEnd})
		leafStart = d
		leafEnd = d
	}
	leaves = append(leaves, emptyLeaf{start: leafStart, end: leafEnd})
	return leaves
}

func (u *PriceIngest) fetchAll(ctx context.Context, start, end time.Time, marketCode string) ([]moa.TradeRecord, []emptyLeaf, error) {
	records, err := u.client.FetchRange(ctx, start, end, "", marketCode)
	if err == nil {
		if len(records) == 0 {
			return nil, []emptyLeaf{{start: start, end: end}}, nil
		}
		return records, nil, nil
	}
	if !errors.Is(err, moa.ErrResponseTruncated) {
		return nil, nil, err
	}
	if isSameDay(start, end) {
		return nil, nil, fmt.Errorf("單日行情（market=%s, date=%s）仍觸及列數上限，無法再切分: %w",
			marketCode, start.Format("2006-01-02"), err)
	}

	mid := midDate(start, end)
	left, leftEmpty, err := u.fetchAll(ctx, start, mid, marketCode)
	if err != nil {
		return nil, nil, err
	}
	right, rightEmpty, err := u.fetchAll(ctx, mid.AddDate(0, 0, 1), end, marketCode)
	if err != nil {
		return nil, nil, err
	}
	return append(left, right...), append(leftEmpty, rightEmpty...), nil
}

func (u *PriceIngest) retryEmptyLeaf(ctx context.Context, start, end time.Time, marketCode string) ([]moa.TradeRecord, bool, error) {
	for attempt := 0; attempt < emptyResponseMaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(u.retryDelay):
		}
		records, _, err := u.fetchAll(ctx, start, end, marketCode)
		if err != nil {
			return nil, false, err
		}
		if len(records) > 0 {
			return records, false, nil
		}
	}
	return nil, true, nil
}

func (u *PriceIngest) fetchAllWithEmptyRetry(
	ctx context.Context,
	checker marketExistenceChecker,
	start, end time.Time,
	marketCode string,
) ([]moa.TradeRecord, []emptyLeaf, error) {
	records, emptyLeaves, err := u.fetchAll(ctx, start, end, marketCode)
	if err != nil {
		return nil, nil, err
	}

	leaves := append([]emptyLeaf{}, emptyLeaves...)
	if len(records) > 0 {
		alreadyCovered := make(map[time.Time]struct{})
		for _, l := range emptyLeaves {
			for d := l.start; !d.After(l.end); d = d.AddDate(0, 0, 1) {
				alreadyCovered[d] = struct{}{}
			}
		}

		scrutinyEnd := end.AddDate(0, 0, -stableLagDays)
		if !scrutinyEnd.Before(start) {
			var missingFiltered []time.Time
			for _, d := range missingCalendarDates(records, start, scrutinyEnd) {
				if _, ok := alreadyCovered[d]; ok {
					continue
				}
				missingFiltered = append(missingFiltered, d)
			}

			leaves = append(leaves, mergeConsecutiveDates(missingFiltered)...)
		}
	}
	if len(leaves) == 0 {
		return records, nil, nil
	}

	hasPrior, checkErr := checker.MarketHasAnyObservation(ctx, gen.MarketHasAnyObservationParams{
		MarketCode: marketCode,
		BeforeDate: pgxutil.DateParam(&end),
	})
	if checkErr != nil {
		zap.L().Warn("price_ingest: 查詢市場過去是否有資料失敗，略過空回應重試判斷",
			zap.String("market_code", marketCode), zap.Error(checkErr))
		return records, nil, nil
	}
	if !hasPrior {
		return records, nil, nil
	}

	var stillEmpty []emptyLeaf
	for _, leaf := range leaves {
		leafRecords, isEmpty, err := u.retryEmptyLeaf(ctx, leaf.start, leaf.end, marketCode)
		if err != nil {
			return nil, nil, err
		}
		if isEmpty {
			stillEmpty = append(stillEmpty, leaf)
			zap.L().Warn("price_ingest: 葉節點重試後仍空手",
				zap.String("market_code", marketCode),
				zap.String("leaf_start", leaf.start.Format("2006-01-02")),
				zap.String("leaf_end", leaf.end.Format("2006-01-02")))
			continue
		}
		records = append(records, leafRecords...)
	}
	return records, stillEmpty, nil
}

func isSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func midDate(start, end time.Time) time.Time {
	days := int(end.Sub(start).Hours() / 24)
	return start.AddDate(0, 0, days/2)
}
