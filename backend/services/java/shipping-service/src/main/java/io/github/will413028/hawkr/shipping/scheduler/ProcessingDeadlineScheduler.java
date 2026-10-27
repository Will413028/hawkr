package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * §5.9 賣家處理期限排程
 *
 * 定期掃描已超過 processing_deadline 但賣家尚未處理（未取號/未叫件）的物流單，
 * 逐筆發布 MQ 事件通知 AfterSales Service 自動取消訂單並退款。
 *
 * 掃描後將 processing_deadline 設為 NULL，防止重複通知。
 *
 * <p><b>搶佔語意（issue #2294 L1）</b>：原本<b>完全沒有搶佔</b>（純 {@code selectList} → publish
 * → {@code clearProcessingDeadline}），多副本下同一張逾期單會被兩個 pod 各通知一次，
 * AfterSales 端收到兩則系統取消+退款。修法＝<b>lease on 既有的
 * {@code shipments.processing_deadline}</b>（候選述詞本身用的就是這一欄，零 migration）：
 * claim 把它推到 {@code now + 5 分鐘}（＝一個 tick），搶到才 publish。
 *
 * <p>⚠️ <b>不是</b>「把 {@code clearProcessingDeadline} 上移到 publish 之前」——那支是
 * {@code SET processing_deadline = NULL}（永久標記），而本方法迴圈內的 catch 只 log 不重拋，
 * 上移後只要 publish 拋例外或 pod 崩潰，{@code shipment.processing.timeout} 就永久遺失且無告警，
 * 等於把 at-least-once 換成 at-most-once（方向與本 issue 要修的病相同）。
 * clear 維持在 publish 之後：它的 {@code IS NOT NULL} 述詞對 leased 值仍成立，一行都不用改。
 *
 * <p>設計書：06 §5.20 / §8.5
 */
@Component
@ConditionalOnProperty(name = "hawkr.shipping.processing-deadline.enabled", havingValue = "true", matchIfMissing = true)
public class ProcessingDeadlineScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ProcessingDeadlineScheduler.class);

    /**
     * issue #2294 L1：租約長度（分）＝ 一個 tick（{@code interval-ms} 預設 300000）。
     *
     * <p>下界：單筆最壞耗時 = 兩次 MQ publish（賣家 meta 的 gRPC 在迴圈<b>外</b>批次做完），遠小於 5 分。
     * <p>上界：租約同時是「pod 在 claim 與 publish 之間崩潰」的恢復窗口，取一個 tick 表示
     * 最多延後一輪就重來。取更大值＝逾期取消+退款被延後更久，且候選窗口無上界不會擋住它，
     * 所以這個上界必須由測試釘住（見 {@code ProcessingDeadlineSchedulerTest}）。
     */
    static final long LEASE_MINUTES = 5L;

    private final ShipmentMapper shipmentMapper;
    private final ShippingEventPublisher eventPublisher;
    private final SellerGrpcClient sellerGrpcClient;

    @Value("${hawkr.shipping.processing-deadline.batch-size:50}")
    private int batchSize;

    public ProcessingDeadlineScheduler(ShipmentMapper shipmentMapper,
                                       ShippingEventPublisher eventPublisher,
                                       SellerGrpcClient sellerGrpcClient) {
        this.shipmentMapper = shipmentMapper;
        this.eventPublisher = eventPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
    }

    /**
     * 預設每 5 分鐘掃描一次
     */
    @Scheduled(fixedDelayString = "${hawkr.shipping.processing-deadline.interval-ms:300000}", initialDelay = 90_000)
    public void scanExpiredShipments() {
        // ⚠️ 這個 now 只用於「候選述詞」與「租約未到期判定」。租約的**基準**不在這裡取
        // （見下方迴圈內），否則整批共用同一個到期時間。
        java.time.LocalDateTime now = DateTimeUtils.nowUtc();
        List<ShipmentEntity> expired = shipmentMapper.findExpiredUnprocessedShipments(
                now, batchSize);

        if (expired.isEmpty()) {
            return;
        }

        log.info("ProcessingDeadlineScheduler.scan.found",
                "發現逾期未處理物流單",
                "count", String.valueOf(expired.size()));

        List<Long> storeIds = expired.stream()
                .map(ShipmentEntity::getStoreId)
                .filter(java.util.Objects::nonNull)
                .distinct()
                .toList();
        Map<Long, Long> resolvedAccountIds = resolveSellerAccountIds(storeIds);

        int notifiedCount = 0;
        for (ShipmentEntity shipment : expired) {
            try {
                // ⚠️ 租約基準取 **claim 當下** 而非 tick 起點的 now。取 tick 起點 ⇒ 全批共用同一個
                // 到期時間 ⇒ 第 k 筆的**有效**租約 = LEASE_MINUTES − (前 k−1 筆的耗時)，不變式退化成
                // 「批次總耗時 < 租約」（會隨 batch-size 一起破）。改在這裡取之後，不變式降為
                // 「**單筆**最壞耗時 < 租約」——與 batchSize 無關，是結構保證而非參數巧合。
                // 述詞用的 now 維持 tick 起點：那個方向讓候選集合與可搶佔集合更小，是保守的。
                java.time.LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(LEASE_MINUTES);
                // issue #2294 L1：lease claim —— 搶到才 publish（0 = 另一個 pod 已搶走）
                int leased = shipmentMapper.leaseProcessingDeadline(shipment.getId(), now, leaseUntil);
                if (leased == 0) {
                    log.info("ProcessingDeadlineScheduler.claim.skip",
                            "租約由他人持有，skip, orderNo=" + shipment.getOrderNo());
                    continue;
                }

                // §5.9.8 先發 cleared 事件清 Order 端 deadline，避免「已逾期但前端仍顯示倒數」暫態
                eventPublisher.publishProcessingCleared(shipment.getOrderNo());

                // 發布逾期事件，AfterSales 觸發系統取消+退款；resolve 失敗時帶 0（issue #2789 階段 B2，
                // 理由見下方 resolveSellerAccountIds 的 javadoc）。
                // 第三參數傳裸 buyer_id（payload key 為 buyerId），消費端只用 orderNo，不需要解析。
                Long resolvedSellerAccountId = resolvedAccountIds.getOrDefault(shipment.getStoreId(), 0L);
                if (!resolvedAccountIds.containsKey(shipment.getStoreId())) {
                    log.warn("ProcessingDeadlineScheduler.resolveSellerAccountId.notFound",
                            "seller-service 沒有回傳這個 store_id 的 meta（可能是不存在的 store_id，"
                                    + "或本輪批次解析整批失敗），sellerAccountId 帶 0 而非回吐 store_id",
                            "storeId", String.valueOf(shipment.getStoreId()));
                }
                eventPublisher.publishProcessingTimeout(
                        shipment.getOrderNo(),
                        resolvedSellerAccountId,
                        shipment.getBuyerId());

                // 清除 deadline 防止重複掃描
                shipmentMapper.clearProcessingDeadline(shipment.getId());
                notifiedCount++;
            } catch (Exception e) {
                log.error("ProcessingDeadlineScheduler.notify.error",
                        "逾期通知失敗, orderNo=" + shipment.getOrderNo(), e);
            }
        }

        log.info("ProcessingDeadlineScheduler.scan.done",
                "逾期通知批次完成，total=" + expired.size() + ", notified=" + notifiedCount);
    }

    /**
     * 批次解析 store_id → 賣家登入 principal account_id。
     *
     * <p><b>解析不到的 id 不會出現在回傳 map 裡</b>，呼叫端據此帶 {@code 0L}（issue #2789 階段 B2），
     * <b>不再回吐 store_id</b>。理由：本值唯一的去處是 {@code shipment.processing.timeout} 的
     * {@code sellerAccountId}，而該事件<b>沒有任何 notification template</b>
     * （唯一消費者 aftersales {@code ProcessingTimeoutConsumer} 只讀 {@code orderNo}）
     * ⇒ 回吐 store_id 帶來零投遞收益，卻讓一個別的身分空間的合法非零值繼續在事件裡流動。
     *
     * <p>整批失敗時回空 map ⇒ 本輪每一筆都帶 0 並各留一行 WARN（逐筆可見，
     * 不再只有 catch 裡的那一行）。
     */
    private Map<Long, Long> resolveSellerAccountIds(List<Long> storeIds) {
        if (storeIds.isEmpty()) {
            return Map.of();
        }
        try {
            BatchGetSellerMetaResponse resp = sellerGrpcClient.batchGetSellerMeta(storeIds);
            return resp.getSellersMap().entrySet().stream()
                    .collect(Collectors.toMap(Map.Entry::getKey, e -> e.getValue().getAccountId()));
        } catch (Exception e) {
            log.warn("resolveSellerAccountIds.error",
                    "批次解析賣家 accountId 失敗，本輪 sellerAccountId 全數帶 0: " + e.getMessage());
            return Map.of();
        }
    }
}
