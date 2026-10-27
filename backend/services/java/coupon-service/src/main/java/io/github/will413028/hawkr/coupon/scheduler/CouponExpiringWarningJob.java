package io.github.will413028.hawkr.coupon.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.coupon.entity.projection.CouponExpiringRef;
import io.github.will413028.hawkr.coupon.event.CouponEventPublisher;
import io.github.will413028.hawkr.coupon.event.CouponExpiringEvent;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.coupon.mapper.CouponInstanceMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.List;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import java.util.Map;

/**
 * 優惠券即將到期預警排程（issue #1176 PR-4）。
 *
 * <p>每日掃描 UNUSED 且所屬 template 於預警窗口（預設 24h）內到期的 instance，per-instance
 * fan-out 發 {@code coupon.expiring} event（snake_case）。冪等由 notification 端 reference_id
 * {@code expiring_{coupon_instance_id}_{date_key}} 承擔（UNIQUE），本 job 重跑 / 多 pod 天然去重，
 * 不引入 dedup 表（docs/開發規範/後端服務責任邊界規範.md）。
 *
 * <p>buyer_id rekey Phase 5（2026-07-18）：{@code coupon_instances.owner_buyer_id} 是
 * stint-scoped buyer_id，notification-service 的 {@code device_tokens} 是登入 account_id
 * 鍵的 —— 發布前用 {@link BuyerGrpcClient#resolvePrincipals} 批次反查每批次的 distinct
 * buyer_id，鏡像 order-service Phase 1 對 {@code notify_buyer_account_id} 的做法。反查失敗
 * （RPC 例外）時本批次全部帶 0（notification-service 端 skip，不阻斷排程本身）；反查成功但
 * 某個 buyer_id 不在回應 map 裡則視為未分岔，fallback 為原值。
 *
 * <h2>搶佔語意（issue #2294 <b>coupon L7</b>）：C-none —— 刻意不加任何 DB 搶佔</h2>
 *
 * <p>⚠️ <b>L 編號是各服務各自的盤點序號，全 repo 不唯一</b>：另有一個 {@code L7} 屬
 * aftersales-service 的 7-11 退貨標籤重印路徑（{@code AfterSaleLabelRetryService}，09 §8.5）。
 * 兩者無關。跨服務引用 L 編號時一律帶服務名（本行即為此）。
 *
 * <p>本 job 是全清單裡<b>唯一</b>結構上無法用租約（lease）的一支，走
 * 《多副本排程搶佔規範》的<b>例外 B</b>：
 *
 * <ul>
 *   <li><b>為什麼租約不成立</b>：候選述詞是 {@code ct.end_at > now AND ct.end_at <= now + warningDays}
 *       （{@code warningDays} 預設 <b>1</b>），而 cron <b>每日只跑一次</b>（{@code 0 0 1 * * ?}）
 *       ⇒ 每張券一生<b>只會落在一個 tick</b> 的候選集合裡。冷卻租約 &lt; 1 天則同一 tick 內不生效、
 *       ≥ 1 天則到期時券早已過期不再是候選 ⇒ <b>任何租約值的行為都與永久旗標相同</b>。
 *       這是「cron 週期 ≥ 候選窗口寬度」這條通則的唯一實例。</li>
 *   <li><b>跨 pod 重複由 consumer 冪等吸收</b>：
 *       {@code UNIQUE INDEX notifications_inbox_dedup_idx ON notifications
 *       (user_id, user_type, event_type, reference_id) WHERE reference_id IS NOT NULL}
 *       配 {@code coupon.expiring} 的 {@code reference_id_template
 *       = 'expiring_#{coupon_instance_id}_#{date_key}'} ⇒ 同一 instance 同一天無論被 publish
 *       幾次，收件匣只會有一則。重複代價 = 0 則多餘通知。</li>
 * </ul>
 *
 * <p>🚨 <b>已知殘留（不是「已修好」）</b>：C-none 只消除<b>跨 pod 重複</b>，
 * <b>完全不改變現況的 at-most-once</b>。例外 B 的定義是「publish → stamp，publish 失敗則下輪重試」，
 * 但本 job <b>既沒有 stamp、也沒有下一輪</b>（每張券一生只落在一個 tick），而
 * {@link CouponEventPublisher#publishCouponExpiring} <b>自帶 try/catch 把例外吞成 log.error</b>
 * ⇒ publish 一失敗，該張券的到期通知<b>永久遺失、無告警、無重試</b>。
 * 修這個遺失需另案，兩條路：① publish 與候選查詢同 tx 走 outbox（coupon 目前無 outbox）
 * ② 把 cron 週期縮短到明顯小於候選窗口（例：{@code warningDays=1} 配 6 小時一跑），
 * 使租約重新成立、退回 lease。<b>本 issue 不做</b>。
 *
 * <p>設計書：07 §7.5
 */
@Component
public class CouponExpiringWarningJob {

    private static final ObsLogger log = ObsLogger.forClass(CouponExpiringWarningJob.class);
    private static final DateTimeFormatter DATE_KEY_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");

    @Value("${coupon.scheduler.expiring-warning.enabled:true}")
    private boolean enabled;

    @Value("${coupon.scheduler.expiring-warning.days:1}")
    private int warningDays;

    @Value("${coupon.scheduler.expiring-warning.batch-size:500}")
    private int batchSize;

    private final CouponInstanceMapper instanceMapper;
    private final CouponEventPublisher publisher;
    private final BuyerGrpcClient buyerGrpcClient;

    public CouponExpiringWarningJob(CouponInstanceMapper instanceMapper, CouponEventPublisher publisher,
                                     BuyerGrpcClient buyerGrpcClient) {
        this.instanceMapper = instanceMapper;
        this.publisher = publisher;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    @Scheduled(cron = "${coupon.scheduler.expiring-warning.cron:0 0 1 * * ?}")
    public void execute() {
        if (!enabled) {
            return;
        }
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime until = now.plusDays(warningDays);
        String dateKey = now.format(DATE_KEY_FMT);
        String occurredAt = DateTimeUtils.formatIso8601(now);

        long cursor = 0L;
        int total = 0;
        try {
            while (true) {
                List<CouponExpiringRef> batch = instanceMapper.scanExpiringUnused(now, until, cursor, batchSize);
                if (batch.isEmpty()) {
                    break;
                }

                List<Long> ownerBuyerIds = batch.stream().map(CouponExpiringRef::ownerBuyerId).distinct().toList();
                PrincipalResolution resolvedPrincipals;
                boolean resolveFailed;
                try {
                    // 通知類：未解析帶 0（略過收件人），不回吐 buyer_id 冒充 account_id。
                    resolvedPrincipals = buyerGrpcClient.resolvePrincipals(ownerBuyerIds);
                    resolveFailed = false;
                } catch (Exception e) {
                    log.error("CouponExpiringWarningJob.execute.resolvePrincipalsFailed",
                            "buyer_id -> account_id 批次解析失敗，本批次通知將帶 0（notification-service 端 skip）", e);
                    resolvedPrincipals = PrincipalResolution.empty();
                    resolveFailed = true;
                }

                for (CouponExpiringRef ref : batch) {
                    long notifyBuyerAccountId = resolveFailed
                            ? 0L
                            : resolvedPrincipals.principalOrZero(ref.ownerBuyerId(), "buyer_id");
                    publisher.publishCouponExpiring(CouponExpiringEvent.of(
                            ref.ownerBuyerId(),
                            notifyBuyerAccountId,
                            ref.instanceId(),
                            ref.couponName(),
                            null,
                            DateTimeUtils.formatIso8601(ref.endAt()),
                            dateKey,
                            occurredAt));
                    cursor = ref.instanceId();
                    total++;
                }
                if (batch.size() < batchSize) {
                    break;
                }
            }
            if (total > 0) {
                log.info("CouponExpiringWarningJob.execute.done", "優惠券到期預警發送完成",
                        "count", String.valueOf(total));
            }
        } catch (Exception e) {
            log.error("CouponExpiringWarningJob.execute.failed", "優惠券到期預警排程異常", e);
        }
    }
}
