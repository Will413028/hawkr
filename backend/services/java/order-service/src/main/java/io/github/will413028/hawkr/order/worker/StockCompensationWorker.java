package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.StockOperationStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.StockTxRecordEntity;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.github.will413028.hawkr.product.grpc.StockItem;
import io.github.will413028.hawkr.product.grpc.StockOperationResponse;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 庫存補償 Worker
 *
 * 處理場景：
 * 1. DeductStock 後崩潰（INIT 記錄，超過 2 分鐘未更新）→ 呼叫 RestoreStock 補償
 * 2. RestoreStock 失敗（FAILED 記錄）→ 重試
 *
 * <p><b>搶佔語意（issue #2294 H5）</b>：兩個迴圈各有一道
 * <b>lease on 既有的 {@code stock_tx_records.next_retry_time}</b> 閘門，
 * 都在副作用（{@code ProductGrpcClient.restoreStock}）之前。零 migration。
 * 詳見 {@code StockTxRecordRepository.claimForCompensation} 與
 * {@code docs/開發規範/多副本排程搶佔規範.md}。
 */
@Component
public class StockCompensationWorker {

    private static final ObsLogger log = ObsLogger.forClass(StockCompensationWorker.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final int BATCH_SIZE = 20;
    private static final int MAX_RETRY_COUNT = OrderConstants.DEFAULT_MAX_RETRY_COUNT;

    /**
     * issue #2294 H5：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：兩個候選述詞
     * （{@code findPendingCompensationByStatus} / {@code findFailedRestoresByStatus}）
     * 都只有下界（{@code created_at < NOW()-2min}、{@code next_retry_time <= NOW()}），
     * <b>沒有時間上界</b> ⇒ 候選窗口寬度 <b>∞</b> ⇒ 任何租約長度都合格；
     * cron 週期 60 秒遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步）：claim 之後唯一的長工作是
     * {@link ProductGrpcClient#restoreStock}，其最壞耗時由
     * {@link ProductGrpcClient#INTERNAL_READ_DEADLINE_SEC}（10 秒）硬性封頂
     * （在此之前該呼叫<b>沒有任何 deadline</b>，下界無從建立）。
     * 取 <b>300 秒</b>（= tick 60 秒 × 5，且 30 倍於最壞耗時）。
     * ⚠️ 這個 300 秒是<b>每一筆</b>的（基準在 {@link #claim} 內取），不是整批共用；
     * 兩個迴圈合計 40 筆 × 10 秒 = 400 秒的批次上界已經超過它。
     *
     * <p>⚠️ 加長租約不會拖慢正常重試：失敗路徑的 {@code updateFailed} 寫的是指數退避值、
     * 且一定在 claim 之後執行，天然覆蓋租約值。租約只影響「pod 在 claim 與 gRPC 之間崩潰」
     * 的救援延遲（由 0 延長為 300 秒）。
     */
    static final long CLAIM_LEASE_SEC = 300L;

    private final StockTxRecordRepository stockTxRecordRepository;
    private final ProductGrpcClient productGrpcClient;

    public StockCompensationWorker(StockTxRecordRepository stockTxRecordRepository,
                                   ProductGrpcClient productGrpcClient) {
        this.stockTxRecordRepository = stockTxRecordRepository;
        this.productGrpcClient = productGrpcClient;
    }

    /**
     * 每 60 秒掃描一次需要補償的記錄
     */
    @Scheduled(fixedDelay = 60000, initialDelay = 30000)
    public void compensate() {
        // 處理 INIT/FAILED 的 DEDUCT 記錄
        List<StockTxRecordEntity> pendingRecords = stockTxRecordRepository.findPendingCompensation(BATCH_SIZE);
        for (StockTxRecordEntity record : pendingRecords) {
            // issue #2294 H5：搶佔閘門在副作用（restoreStock gRPC）之前；回 0 即 skip，
            // 一個副作用都不做（含 markManualIntervention）。
            if (!claim(record)) {
                continue;
            }
            processCompensation(record);
        }

        // 處理 FAILED 的 RESTORE 記錄
        List<StockTxRecordEntity> failedRestores = stockTxRecordRepository.findFailedRestores(BATCH_SIZE);
        for (StockTxRecordEntity record : failedRestores) {
            if (!claim(record)) {
                continue;
            }
            retryRestore(record);
        }
    }

    /**
     * lease CAS 搶佔一筆記錄。
     *
     * <p>⚠️ <b>R4-P1-2（2026-08-08）：租約基準在這裡取，不在 {@link #compensate} 開頭取。</b>
     * 原本 {@code leaseUntil} 在<b>兩個迴圈之前</b>只算一次 ⇒ 全批共用同一個到期時間
     * ⇒ 第 k 筆的有效租約 = {@link #CLAIM_LEASE_SEC} − (前 k−1 筆的耗時)。
     * 批次上界 = {@code (20 + 20) 筆 × 10 秒 deadline = 400 秒 > 300 秒租約}
     * ⇒ 跑到後段時租約已過期，另一個 pod 會在原持有者仍在打 gRPC 時合法搶走同一筆。
     *
     * @return true = 搶到，呼叫端可以往下做副作用；false = 已被另一個 pod 認領
     */
    private boolean claim(StockTxRecordEntity record) {
        LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC);
        int claimed = stockTxRecordRepository.claimForCompensation(
                record.getStockTxId(), record.getPhase(), leaseUntil);
        if (claimed == 0) {
            log.info("StockCompensationWorker.claim.skipped",
                    "本筆已被另一個 pod 認領，skip phase=" + record.getPhase(),
                    "stockTxId", record.getStockTxId());
            return false;
        }
        return true;
    }

    private void processCompensation(StockTxRecordEntity record) {
        // 停損檢查
        if (record.getRetryCount() >= MAX_RETRY_COUNT) {
            stockTxRecordRepository.markManualIntervention(
                    record.getStockTxId(), record.getPhase());
            log.info("StockCompensationWorker.manualIntervention",
                    "超過最大重試次數，轉人工處理: " + record.getPhase(),
                    "stockTxId", record.getStockTxId());
            return;
        }

        log.info("StockCompensationWorker.process.start", "處理庫存補償",
                "stockTxId", record.getStockTxId());

        List<StockItem> items = parsePayload(record.getPayloadSnapshot());
        if (items.isEmpty()) {
            log.error("StockCompensationWorker.process.emptyPayload", "payload 為空",
                    "stockTxId", record.getStockTxId());
            return;
        }

        try {
            StockOperationResponse response = productGrpcClient.restoreStock(record.getStockTxId(), items);

            boolean restored = StockOperationStatus.SUCCESS.value().equals(response.getStatus())
                    || StockOperationStatus.ALREADY_APPLIED.value().equals(response.getStatus());
            boolean noop = StockOperationStatus.NOOP_NOT_FOUND.value().equals(response.getStatus());

            if (restored || noop) {
                // restored：庫存已歸還 → 冪等寫 RESTORE 記錄（防 PK DuplicateKey，03 §8.1）。
                // noop：因果閘門判定從未扣減 → 無庫存被還，不寫 RESTORE 記錄。
                if (restored) {
                    stockTxRecordRepository.insertRestoreIfAbsent(record.getStockTxId(), record.getOrderNo(),
                            OrderConstants.STOCK_TX_PHASE_RESTORE, OrderConstants.STOCK_TX_STATUS_SUCCESS, "{}");
                }

                // DEDUCT 改終態 COMPENSATED → 退出掃描範圍 B，不再重撈（修死循環根因，03 §8.1）
                stockTxRecordRepository.updateStatus(record.getStockTxId(),
                        OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_COMPENSATED);

                log.info("StockCompensationWorker.process.success",
                        "庫存補償成功 restoreStatus=" + response.getStatus(),
                        "stockTxId", record.getStockTxId());
            } else {
                int retryDelay = calculateRetryDelay(record.getRetryCount());
                stockTxRecordRepository.updateFailed(record.getStockTxId(),
                        OrderConstants.STOCK_TX_PHASE_DEDUCT,
                        OrderConstants.STOCK_TX_STATUS_FAILED, "RestoreStock 回傳: " + response.getStatus(), retryDelay);
            }
        } catch (Exception e) {
            log.error("StockCompensationWorker.process.error", "庫存補償 gRPC 失敗", e);
            int retryDelay = calculateRetryDelay(record.getRetryCount());
            stockTxRecordRepository.updateFailed(record.getStockTxId(),
                    OrderConstants.STOCK_TX_PHASE_DEDUCT,
                    OrderConstants.STOCK_TX_STATUS_FAILED, e.getMessage(), retryDelay);
        }
    }

    private void retryRestore(StockTxRecordEntity record) {
        // 停損檢查
        if (record.getRetryCount() >= MAX_RETRY_COUNT) {
            stockTxRecordRepository.markManualIntervention(
                    record.getStockTxId(), record.getPhase());
            log.info("StockCompensationWorker.manualIntervention",
                    "超過最大重試次數，轉人工處理: " + record.getPhase(),
                    "stockTxId", record.getStockTxId());
            return;
        }

        log.info("StockCompensationWorker.retryRestore.start", "重試庫存歸還",
                "stockTxId", record.getStockTxId());

        // 從同一 stockTxId 的 DEDUCT 記錄取得 payload
        StockTxRecordEntity deductRecord = stockTxRecordRepository.findByStockTxIdAndPhase(
                record.getStockTxId(), OrderConstants.STOCK_TX_PHASE_DEDUCT);
        if (deductRecord == null) {
            log.error("StockCompensationWorker.retryRestore.noOriginal", "找不到原始扣減記錄");
            return;
        }

        List<StockItem> items = parsePayload(deductRecord.getPayloadSnapshot());
        try {
            StockOperationResponse response = productGrpcClient.restoreStock(record.getStockTxId(), items);
            if (StockOperationStatus.SUCCESS.value().equals(response.getStatus())
                    || StockOperationStatus.ALREADY_APPLIED.value().equals(response.getStatus())
                    || StockOperationStatus.NOOP_NOT_FOUND.value().equals(response.getStatus())) {

                stockTxRecordRepository.updateFailed(record.getStockTxId(),
                        OrderConstants.STOCK_TX_PHASE_RESTORE,
                        OrderConstants.STOCK_TX_STATUS_SUCCESS, null, 0);
                log.info("StockCompensationWorker.retryRestore.success", "重試歸還成功");
            } else {
                int retryDelay = calculateRetryDelay(record.getRetryCount());
                stockTxRecordRepository.updateFailed(record.getStockTxId(),
                        OrderConstants.STOCK_TX_PHASE_RESTORE,
                        OrderConstants.STOCK_TX_STATUS_FAILED, "回傳: " + response.getStatus(), retryDelay);
            }
        } catch (Exception e) {
            log.error("StockCompensationWorker.retryRestore.error", "重試歸還失敗", e);
            int retryDelay = calculateRetryDelay(record.getRetryCount());
            stockTxRecordRepository.updateFailed(record.getStockTxId(),
                    OrderConstants.STOCK_TX_PHASE_RESTORE,
                    OrderConstants.STOCK_TX_STATUS_FAILED, e.getMessage(), retryDelay);
        }
    }

    private List<StockItem> parsePayload(String json) {
        try {
            JsonNode items = MAPPER.readTree(json);
            return java.util.stream.StreamSupport.stream(items.spliterator(), false)
                    .map(node -> StockItem.newBuilder()
                            .setSkuId(String.valueOf(node.path("skuId").asLong()))
                            .setQuantity(String.valueOf(node.path("quantity").asInt()))
                            .build())
                    .toList();
        } catch (JsonProcessingException e) {
            log.error("StockCompensationWorker.parsePayload.error", "解析 payload 失敗", e);
            return List.of();
        }
    }

    /**
     * 指數退避（最大 3600 秒）
     */
    private int calculateRetryDelay(int retryCount) {
        return Math.min(60 * (1 << Math.min(retryCount, 6)), 3600);
    }
}
