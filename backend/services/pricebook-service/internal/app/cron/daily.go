// Package cron 是 pricebook-service 的每日排程：先跑休市日曆同步，再跑行情
// ingest，跑在服務自身行程內（無 k8s CronJob、無 gRPC 觸發，見 decisions.md D1）。
package cron

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// 每日執行時刻（UTC）。20:00 UTC 是本服務目前的空檔——同 repo 既有 daily job
// 已佔用 19:00 / 19:05 / 19:30 / 19:40 UTC（seller-service 的
// governance_jobs.go / di.go），20:00 UTC 尚未被使用。常數以 UTC 命名，
// 避免與容器本地時區混淆（compose 明確設 TZ=UTC；prod image 未設 TZ 時
// 同樣預設 UTC）。
const (
	dailyHourUTC   = 20
	dailyMinuteUTC = 0

	// 補抓輪次（priceOnly）。MOA 的上架時間散在台北白天，而完整排程跑在
	// 20:00 UTC ＝ 台北 04:00——當天稍晚才上架的格子要等整整 24 小時才會
	// 被再問一次。這兩個時刻把最壞等待從 24 小時壓到約 8 小時。
	//
	// 選 06:00／12:00 UTC（台北 14:00／20:00）的理由：兩者都落在 MOA 白天
	// 上架之後，且避開本 repo 已佔用的 19:00／19:05／19:30／19:40／20:00 UTC
	// 那一排 daily job，不跟它們搶同一個時間點。
	catchupMinuteUTC = 0
)

// catchupHoursUTC 是補抓輪次的時刻（UTC）。放 var 而非 const 只因為 Go 的
// const 不能是陣列；它在執行期不被修改。
var catchupHoursUTC = []int{6, 12}

// 🪦 windowDays（已移除，2026-09-02）——這裡留一段墓誌銘，因為它的失敗方式
// 值得記住。
//
// 它曾是「抓取區間 = 今天 - windowDays ~ 今天」的滾動窗口，同時身兼兩個角色：
// (a) 吸收 MOA 的上架延遲、(b) 讓排程漏跑一天可以自我修復。問題在於它是**盲**
// 的——每天重抓一個固定寬度的範圍，從不知道自己缺什麼；一格資料只要延遲超過
// 窗口寬度才上架，就會滑出去，之後再也不會被抓，而且沒有任何東西會發現。
// 2026-09-02 實測 dev 有 7 個這樣的永久空洞，其中 2026-07-24 是 21 個市場全缺
// （上游至今查得到資料）。它的寬度也一直是猜的：3 → 6 的依據是「當時實測最長
// 延遲 4 天」，而後來量到的分佈尾巴有 11 天。
//
// 兩個角色現在各自有主：(a) 由 ingest_partition 的 attempt_count /
// MaxIngestAttempts 承接——「多晚算太晚」變成寫下來的決定，而不是窗口寬度的
// 副作用；(b) 由 declareLookbackDays 承接（見 usecase/price_partition.go）。
// 差別是漏掉的格子會留成 pending 直到補上或明確 abandoned，不會因為時間過去
// 就消失。

// dailyRunTimeout 是每次 daily job 呼叫（休市日曆同步 + 行情 ingest）的上限。
//
// pkg/outbox.DailyScheduler 的排程迴圈是嚴格序列執行（算下一次時間點前一定
// 等 run closure 回傳），MOA client 本身的 http.Client{Timeout: 60s}
// （見 internal/client/moa/client.go）只界定「單一次 HTTP 請求」，界定不到
// 「一次完整 daily job 呼叫」——一次呼叫內含多次 HTTP 呼叫：RestDayIngest.Handle
// 一次（休市日曆，內含 ~21 個市場 × 多年份的休市日 upsert）；
// PriceIngest.Handle 對每個市場至少一次 FetchRange，觸頂時對半切遞迴（見
// decisions.md D8 的既有結論：單日單市場不會觸頂，月範圍會）。
//
// 🔴 2026-08-17 的實測仍是這個預算的依據：當時 windowDays（已移除，見上方
// 墓誌銘）從 3 改成 6，用 cmd/pricebook-backfill CLI 對 dev 實測 7 天、
// 21 個市場全跑，總耗時 8 分 58 秒——遠低於 30 分鐘預算。
//
// 分區模型下單次執行的工作量上界不再是「固定天數 × 市場數」，而是「累積的
// pending 格數」：穩態下遠小於當年那個 7×21，真正的上界情境是長期停擺後一次
// 補回來——而那種時候該看的是 MaxIngestAttempts 有沒有先把格子判掉，不是把
// 這個 timeout 加大。若要重新校準，同樣用 backfill CLI 跑一次真實 dry-run
// 取數字，不要沿用紙上推算。
//
// 30 分鐘本身維持不變：正常執行（實測數十秒到十分鐘量級）留足緩衝，同時
// MOA 全面無回應時不會無限期卡住排程迴圈，讓下一次 20:00 UTC 排程仍能準時
// 進入下一輪等待。
const dailyRunTimeout = 30 * time.Minute

// finishIngestRunTimeout 是 FinishIngestRun 這通「收尾寫入」自己的逾時預算。
//
// 🔴 刻意用 context.WithoutCancel(ctx) 剝離掉 run 本身的 dailyRunTimeout
// deadline：FinishIngestRun 正是「這一輪因為 ctx 到期或任何其他錯誤而失敗」
// 之後，把這個失敗結果寫回 ingest_run 的最後一步——如果沿用同一個已經到期的
// ctx，這通收尾寫入會立刻連帶失敗，留下一列卡在 status=running、
// error_message 是 NULL 的稽核紀錄，而「這輪到底發生了什麼」就只剩下易失的
// log，正是本檔案上方（run 的說明）想避免的情況：「log 是易失的，ingest_run
// 才是 Task 9 查詢連續失敗會看的稽核紀錄」在逾時這條路徑上會整個倒過來。
// OnStop 呼叫 cancel() 撞上 run 尾聲時是同一個問題，同一個修法解決。
// 10 秒是給這通單一 UPDATE 的合理上限：連 10 秒都撐不住通常代表資料庫本身
// 有狀況，那種情況下記 log 才是唯一能做的事。
const finishIngestRunTimeout = 10 * time.Second

// restDayIngester 是 daily job 對休市日曆同步需要的最小介面（比照
// services/seller-service/internal/app/cron/payuni_status_history_cleanup.go
// 的慣例：cron 套件宣告自己需要的單一方法介面，不直接依賴 usecase 具體型別，
// 方便測試注入 stub、不用起整個 usecase fx 圖）。*usecase.RestDayIngest
// 滿足此介面。
type restDayIngester interface {
	Handle(ctx context.Context) (int, error)
}

// priceIngester 是 daily job 對行情 ingest 需要的最小介面，同上。
// *usecase.PriceIngest 滿足此介面。
//
// 兩段式：先宣告「應該要有哪些 (市場,交易日) 格子」，再補齊所有還沒拿到的。
// 排程不再自己決定抓取區間——工作清單住在 ingest_partition，這是「滑出窗口
// 就永遠不會被抓」那個失效模式的修法，見 usecase/price_partition.go 檔頭。
type priceIngester interface {
	DeclareDue(ctx context.Context, from, through time.Time) error
	HandleDue(ctx context.Context) (usecase.IngestStats, error)
}

type porkIngester interface {
	Handle(ctx context.Context, date time.Time) (usecase.PorkIngestStats, error)
}

type poultryIngester interface {
	Handle(ctx context.Context, start, end time.Time) (usecase.PoultryIngestStats, error)
}

type riceIngester interface {
	Handle(ctx context.Context, days, skip int) (usecase.RiceIngestStats, error)
}

// ingestRunRecorder 是 daily job 對 ingest_run 稽核紀錄需要的最小介面。
// *gen.Queries 滿足此介面（方法簽章逐字對應 sqlc 產出的
// CreateIngestRun/FinishIngestRun）。抽出介面純粹是為了讓 dailyJob 的核心
// 邏輯（RestDayIngest 失敗不擋 PriceIngest、兩者都失敗要記錄成 Failed、
// 行情窗口的起訖時間）可以用 fake recorder 做單元測試，不需要真的 Postgres。
type ingestRunRecorder interface {
	CreateIngestRun(ctx context.Context, arg gen.CreateIngestRunParams) (int64, error)
	FinishIngestRun(ctx context.Context, arg gen.FinishIngestRunParams) error
}

// dailyJob 是每日排程的實際工作內容。
//
// 🔴 刻意不呼叫 usecase.Backfill.Handle：Backfill.Handle 內部寫死
// domain.IngestKindBackfill、並用 MonthlyChunks 做「日曆月」粒度的切分——
// 兩者的語意都不合每日宣告區間（見 declareLookbackDays）。本 job 直接組合
// RestDayIngest.Handle 與 PriceIngest.Handle，自己做一輪 ingest_run 稽核
// （CreateIngestRun/FinishIngestRun 的呼叫形狀比照
// internal/usecase/backfill.go 的 Backfill.Handle，唯一差異是 kind 換成
// IngestKindDaily、不做月切分）。
type dailyJob struct {
	restDay        restDayIngester
	price          priceIngester
	pork           porkIngester
	poultry        []poultryIngester
	rice           riceIngester
	tsid           func() int64
	queriesFactory func(ctx context.Context) ingestRunRecorder
	logger         *zap.Logger
	now            func() time.Time

	// priceOnly 為真時，這一輪只做「宣告分區 + 消費待補分區」，跳過休市日曆
	// 與肉品／禽蛋／米價。補抓排程（06:00／12:00 UTC）用它。
	//
	// 補抓存在的理由：MOA 的上架時間散在台北白天，而排程原本一天只跑一次
	// （20:00 UTC ＝ 台北 04:00）。2026-09-13 那輪跑的時候 09-12 的資料還沒上架，
	// 抓到 0 筆；當天稍晚就有了，我方卻要等隔天 04:00 才會再問——實測讓約 3,500
	// 列已可取得的資料多躺了將近一天。HandleDue 在沒有待補分區時第一件事就是
	// early return，所以空跑幾乎零成本。
	//
	// 仍然呼叫 DeclareDue：跨 UTC 日界時會有新的「昨天」要宣告，而它本身冪等。
	priceOnly bool
}

// poultryFor 在補抓輪次回傳空清單——家禽跟肉品／米價一樣，一天一次就夠。
// 寫成方法而不是在迴圈前加 if，是為了讓 range 那行維持單一形狀。
func (j *dailyJob) poultryFor(priceOnly bool) []poultryIngester {
	if priceOnly {
		return nil
	}
	return j.poultry
}

// newDailyJob 建構 dailyJob。queriesFactory 預設每次呼叫時用
// gen.New(database.GetDBTX(ctx, pool)) 現組 *gen.Queries——不能把 *gen.Queries
// 本身存成欄位當 singleton，理由與 RestDayIngest/PriceIngest/Backfill 的建構子
// 註解相同：這樣 database.TxManager.WithTx 開的交易才會被自動撿起來。
func newDailyJob(restDay restDayIngester, price priceIngester, pork porkIngester, poultry []poultryIngester, rice riceIngester, pool *pgxpool.Pool, tsid func() int64, logger *zap.Logger) *dailyJob {
	return &dailyJob{
		restDay: restDay,
		price:   price,
		pork:    pork,
		poultry: poultry,
		rice:    rice,
		tsid:    tsid,
		queriesFactory: func(ctx context.Context) ingestRunRecorder {
			return gen.New(database.GetDBTX(ctx, pool))
		},
		logger: logger,
		now:    time.Now,
	}
}

// runWithTimeout 是傳給 outbox.NewDailyScheduler 的 run closure：只包住
// dailyJob.run 的單次執行逾時（dailyRunTimeout），這是本 job 特有的業務邏輯
// ——pkg/outbox.DailyScheduler 不提供這種能力。panic recovery 不再是這裡的
// 責任：DailyScheduler.tick() 自己的 defer/recover 已經包住整個 run closure
// 的呼叫（見 pkg/outbox/daily_scheduler.go），一旦 run 內部 panic（例如 MOA
// JSON 解析或 pgtype 轉換），DailyScheduler 會記錄並吸收它，排程迴圈完好存
// 活，不會讓一次 panic 拖垮整個 pod——效果與舊版本地 runOnceWithRecover 同時
// 兼顧 panic-recovery 與 timeout 相同，只是 recovery 這層現在疊在呼叫端
// （DailyScheduler），不再疊在這裡，兩層 recover 因此收斂成一層。
func (j *dailyJob) runWithTimeout(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, dailyRunTimeout)
	defer cancel()

	j.run(runCtx)
}

// run 是一次完整的每日排程執行：先跑休市日曆（讓當日的休市判定在行情 ingest
// 前就緒），再宣告攝取分區並補齊所有待補的格子，全程自己記錄一筆
// ingest_run。
//
// 兩個步驟的失敗都只記 log、不中斷、不 panic、不往外傳播：
//   - RestDayIngest.Handle 失敗不擋 PriceIngest.Handle——有部分行情資料
//     加上過期的休市日曆，好過什麼都不做。
//   - 任一步驟失敗都會讓這筆 ingest_run 標記為 IngestStatusFailed（合併兩者
//     的錯誤訊息），而不是悄悄吞掉、只留 log（log 是易失的，ingest_run 才是
//     Task 9 查詢連續失敗會看的稽核紀錄——這也是為什麼收尾的 FinishIngestRun
//     要用剝離過 deadline 的 ctx，見 finishIngestRunTimeout 的說明：不這樣做
//     的話，最需要被記下來的那個失敗——ctx 逾時——反而會讓收尾寫入自己也連帶
//     失敗，倒果為因）。
//   - 這裡不做任何重試/backoff/告警：連續失敗的偵測與告警是查詢 ingest_run
//     的責任（Task 9 或人工），不在這支排程內建 reconciliation job（違反
//     服務責任邊界規範，見 decisions.md）。
func (j *dailyJob) run(ctx context.Context) {
	q := j.queriesFactory(ctx)

	now := j.now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// 宣告區間：到「昨天」為止——今天的行情上游還沒出，宣告了只會白白消耗
	// 一次 attempt。ingest_run 的 range 也記這個區間，它現在的語意是「這次
	// 宣告涵蓋哪幾天」，不再是「這次抓了哪幾天」（實際抓的是工作清單）。
	start := today.AddDate(0, 0, -usecase.DeclareLookbackDays)
	end := today.AddDate(0, 0, -1)

	// CreateIngestRun 沿用 run 本身的 ctx（含 dailyRunTimeout 全額 deadline）：
	// 這通呼叫是 run 一開始就做的第一件事，前面還沒有任何耗時的休市日曆/行情
	// 呼叫吃掉預算，30 分鐘幾乎全額可用，不像 FinishIngestRun 是在「deadline
	// 可能剛好已經到期」那個時間點才執行。若它仍然失敗（例如 DB 本身斷線），
	// 沒有 run id 可以掛任何狀態，記 log 已是唯一能做的事——不需要、也沒有
	// 「收尾」動作可以搶救。
	runID := j.tsid()
	if _, err := q.CreateIngestRun(ctx, gen.CreateIngestRunParams{
		ID:         runID,
		Kind:       domain.IngestKindDaily,
		RangeStart: pgxutil.DateParam(&start),
		RangeEnd:   pgxutil.DateParam(&end),
		Status:     domain.IngestStatusRunning,
		StartedAt:  pgxutil.TimestamptzParam(now),
	}); err != nil {
		// 連 ingest_run 都建不起來（通常代表 DB 本身有狀況）：沒有 run id
		// 可以掛失敗狀態，只能記 log。
		j.logger.Error("daily_ingest: 建立 ingest_run 失敗", zap.Error(err))
		return
	}

	var errs []error

	// priceOnly（補抓輪次）跳過休市日曆：它一天只需要同步一次，而補抓的目的
	// 只是把上游稍晚上架的行情格子早點撿回來。
	if !j.priceOnly {
		if written, err := j.restDay.Handle(ctx); err != nil {
			j.logger.Error("daily_ingest: 休市日曆同步失敗", zap.Error(err))
			errs = append(errs, fmt.Errorf("休市日曆同步: %w", err))
		} else {
			j.logger.Info("daily_ingest: 休市日曆同步完成", zap.Int("written", written))
		}
	}

	// 宣告失敗就不消費：工作清單不完整時去抓，會把「還沒宣告」誤當成「沒有
	// 要抓的」，記成一次乾淨的成功執行——正是這次重構要消滅的失敗形狀。
	// 不提早 return：仍要走完下面的收尾，讓這次失敗留在 ingest_run 上。
	var stats usecase.IngestStats
	if err := j.price.DeclareDue(ctx, start, end); err != nil {
		j.logger.Error("daily_ingest: 攝取分區宣告失敗", zap.Error(err))
		errs = append(errs, fmt.Errorf("攝取分區宣告: %w", err))
	} else if s, err := j.price.HandleDue(ctx); err != nil {
		j.logger.Error("daily_ingest: 行情 ingest 失敗", zap.Error(err))
		errs = append(errs, fmt.Errorf("行情 ingest: %w", err))
	} else {
		stats = s
		j.logger.Info("daily_ingest: 行情 ingest 完成",
			zap.Int("fetched", stats.Fetched),
			zap.Int("inserted", stats.Inserted),
			zap.Int("revised", stats.Revised),
			zap.Int("skipped", stats.Skipped))
	}

	if j.pork != nil && !j.priceOnly {
		var porkInserted, porkRevised, porkFailed int
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			ps, err := j.pork.Handle(ctx, d)
			if err != nil {
				porkFailed++
				j.logger.Warn("daily_ingest: 毛豬行情 ingest 失敗",
					zap.String("trade_date", d.Format("2006-01-02")), zap.Error(err))
				continue
			}
			porkInserted += ps.Inserted
			porkRevised += ps.Revised
		}
		if porkFailed > 0 {
			errs = append(errs, fmt.Errorf("毛豬行情 ingest: %d 天失敗", porkFailed))
		}
		j.logger.Info("daily_ingest: 毛豬行情 ingest 完成",
			zap.Int("inserted", porkInserted),
			zap.Int("revised", porkRevised),
			zap.Int("failed_days", porkFailed))
	}

	for _, ing := range j.poultryFor(j.priceOnly) {
		ps, err := ing.Handle(ctx, start, end)
		if err != nil {
			j.logger.Warn("daily_ingest: 家禽行情 ingest 失敗", zap.Error(err))
			errs = append(errs, fmt.Errorf("家禽行情 ingest: %w", err))
			continue
		}
		j.logger.Info("daily_ingest: 家禽行情 ingest 完成",
			zap.Int("days", ps.Days), zap.Int("inserted", ps.Inserted),
			zap.Int("revised", ps.Revised), zap.Int("skipped", ps.Skipped),
			zap.Int("absent", ps.Absent))
	}

	if j.rice != nil && !j.priceOnly {
		rs, err := j.rice.Handle(ctx, usecase.DeclareLookbackDays+1, 0)
		if err != nil {
			j.logger.Warn("daily_ingest: 糧價 ingest 失敗", zap.Error(err))
			errs = append(errs, fmt.Errorf("糧價 ingest: %w", err))
		} else {
			j.logger.Info("daily_ingest: 糧價 ingest 完成",
				zap.Int("fetched", rs.Fetched), zap.Int("days", rs.Days),
				zap.Int("inserted", rs.Inserted), zap.Int("revised", rs.Revised),
				zap.Int("skipped", rs.Skipped), zap.Int("absent", rs.Absent),
				zap.Int("unpublished", rs.Unpublished), zap.Int("incomplete", rs.Incomplete))
		}
	}

	finishedAt := j.now().UTC()
	finish := gen.FinishIngestRunParams{
		ID:            runID,
		FetchedCount:  int32(stats.Fetched),
		InsertedCount: int32(stats.Inserted),
		RevisedCount:  int32(stats.Revised),
		FinishedAt:    &finishedAt,
	}
	attention := stats.AttentionReason()
	switch {
	case len(errs) > 0:
		finish.Status = domain.IngestStatusFailed
		message := errors.Join(errs...).Error()
		finish.ErrorMessage = &message
	case attention != "":
		// 抓到了列、卻一列都沒落地：這一輪沒報錯，但結果等同沒有更新行情。
		// 記成獨立狀態而非 Succeeded，否則這種故障只會留在逐列 log 裡，
		// 稽核表上與「今天本來就沒有行情」完全無法區分。
		finish.Status = domain.IngestStatusNeedsAttention
		finish.ErrorMessage = &attention
	default:
		finish.Status = domain.IngestStatusSucceeded
	}

	// 🔴 刻意不沿用 ctx（見 finishIngestRunTimeout 的說明）：ctx 可能就是導致
	// 這一輪失敗的原因本身（dailyRunTimeout 逾時、或 OnStop 的 cancel()
	// 撞上尾聲）。沿用一個已經死掉的 ctx 呼叫 FinishIngestRun，會讓「記錄這次
	// 逾時失敗」這個收尾動作自己也連帶失敗，剛好是最需要被記下來的那個情境
	// 反而記不下來。q 不需要重組——GetDBTX 只在 ctx 帶有進行中的交易時才回傳
	// 該交易（本方法從不開交易），沒有交易時回傳的是 *pgxpool.Pool 本身，
	// 每次呼叫各自吃自己傳入的 ctx，跟建構 q 當下用的 ctx 無關。
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), finishIngestRunTimeout)
	defer finishCancel()
	if err := q.FinishIngestRun(finishCtx, finish); err != nil {
		// ingest_run 停在 status=running：下次人工排查會看到一列卡住的
		// running 紀錄，而不是誤判成功——比悄悄吞掉這個錯誤更誠實
		// （比照 internal/usecase/backfill.go 的 Handle 對同一情境的取捨）。
		j.logger.Error("daily_ingest: 標記 ingest_run 完成狀態失敗",
			zap.Int64("run_id", runID), zap.Error(err))
	}
}
