package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * §5.3.10.3 退貨 24 小時預警 Scheduler（2026-05-22 新增；2026-06-15 加入待寄回分支）。
 *
 * <p>每 5 分鐘掃描三類 case，發送剩 24h 預警 FCM：
 * <ul>
 *   <li>{@code status='26' AND warned_store_pickup_24h_at IS NULL AND store_pickup_deadline_at 在 (now, now+24h]}
 *       —— 超商待取，推給賣家</li>
 *   <li>{@code status='27' AND warned_confirm_receipt_24h_at IS NULL AND auto_confirm_at 在 (now, now+24h]}
 *       —— 確認收貨，推給賣家</li>
 *   <li>{@code status='20' AND warned_ship_back_24h_at IS NULL AND ship_back_deadline_at 在 (now, now+24h]}
 *       —— 待寄回 D-1，推給買家（2026-06-15 新增）</li>
 * </ul>
 *
 * <p><b>搶佔語意：例外 B（issue #2294 A1–A3，見《多副本排程搶佔規範》§四）</b>——本排程<b>不加</b>
 * 任何 DB 搶佔，順序是 <b>publish → 成功後才 stamp</b>：
 * <ul>
 *   <li>{@code warned_*_24h_at} 已<b>不是</b>冪等閘門，只是<b>事後記錄</b>（成功發過的下次不必再撈，屬效率優化）。
 *       因此 {@code NULL} 不再保證「沒發過」——stamp 失敗時就會是 NULL 但已發。</li>
 *   <li>publish 失敗（{@code tryPublishCaseTransitionWithContext} 回 false）即<b>不 stamp</b>，
 *       下一輪重發——這正是換順序要換到的東西（舊順序下 publish 失敗＝該筆預警永久遺失）。</li>
 *   <li>跨 pod／重複 tick 的重複投遞由 notification 端 {@code notifications_inbox_dedup_idx}
 *       （{@code reference_id = case_return_*_24h_#{case_no}}，不含日期）吸收，收件匣仍只有一則。</li>
 * </ul>
 * ⚠️ 不要把這個順序推廣到本服務其他排程：例外 B 只授權移動「事後記錄」，
 * 移動業務狀態轉換會讓 CAS affected=0 變成「事件說了一件沒發生的事」（設計書 09 §5.3.10.5）。
 */
@Component
public class ReturnTwentyFourHourWarnScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ReturnTwentyFourHourWarnScheduler.class);

    private static final int BATCH_LIMIT = 200;
    private static final long WARN_AHEAD_HOURS = 24;

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSalesEventPublisher eventPublisher;

    public ReturnTwentyFourHourWarnScheduler(AfterSaleCaseMapper caseMapper,
                                              AfterSalesEventPublisher eventPublisher) {
        this.caseMapper = caseMapper;
        this.eventPublisher = eventPublisher;
    }

    @Scheduled(cron = "${aftersales.return.warn-scheduler-cron:0 */5 * * * ?}")
    public void checkWarnings() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime cutoff = now.plusHours(WARN_AHEAD_HOURS);

        scanStorePickupWarn(now, cutoff);
        scanConfirmReceiptWarn(now, cutoff);
        scanShipBackWarn(now, cutoff); // 2026-06-15 待寄回 D-1 預警
    }

    private void scanStorePickupWarn(LocalDateTime now, LocalDateTime cutoff) {
        List<String> candidates = caseMapper.findStorePickup24hWarnCandidates(now, cutoff, BATCH_LIMIT);
        if (candidates.isEmpty()) return;

        log.info("ReturnTwentyFourHourWarnScheduler.storeWarn.found",
                "發現 " + candidates.size() + " 筆 超商待取剩 24h 預警候選");

        int sent = 0;
        for (String caseNo : candidates) {
            try {
                AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                        new LambdaQueryWrapper<AfterSaleCaseEntity>()
                                .eq(AfterSaleCaseEntity::getCaseNo, caseNo));
                if (caseEntity == null) {
                    continue;
                }
                Map<String, Object> extra = new HashMap<>();
                extra.put("shipping_method", caseEntity.getReturnShippingMethod());
                extra.put("deadline_at",
                        caseEntity.getStorePickupDeadlineAt() != null
                                ? caseEntity.getStorePickupDeadlineAt().toString() : null);
                // 例外 B：先 publish，成功才 stamp（publish 失敗 → 不 stamp → 下一輪重發）
                boolean published = eventPublisher.tryPublishCaseTransitionWithContext(
                        AfterSalesEventType.CASE_RETURN_AT_STORE_24H.value(),
                        MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE_24H,
                        caseNo, caseEntity.getOrderNo(),
                        caseEntity.getBuyerId(), caseEntity.getStoreId(),
                        "system", extra);
                if (!published) {
                    log.warn("ReturnTwentyFourHourWarnScheduler.storeWarn.publishFailed",
                            "publish 失敗，不 stamp，下一輪重發, caseNo=" + caseNo);
                    continue;
                }
                caseMapper.stampStorePickup24hWarned(caseNo, DateTimeUtils.nowUtc());
                sent++;
            } catch (Exception e) {
                log.error("ReturnTwentyFourHourWarnScheduler.storeWarn.error",
                        "處理超商待取 24h 預警異常, caseNo=" + caseNo, e);
            }
        }

        log.info("ReturnTwentyFourHourWarnScheduler.storeWarn.done", "送出 FCM 數=" + sent);
    }

    private void scanConfirmReceiptWarn(LocalDateTime now, LocalDateTime cutoff) {
        List<String> candidates = caseMapper.findConfirmReceipt24hWarnCandidates(now, cutoff, BATCH_LIMIT);
        if (candidates.isEmpty()) return;

        log.info("ReturnTwentyFourHourWarnScheduler.confirmWarn.found",
                "發現 " + candidates.size() + " 筆 確認收貨剩 24h 預警候選");

        int sent = 0;
        for (String caseNo : candidates) {
            try {
                AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                        new LambdaQueryWrapper<AfterSaleCaseEntity>()
                                .eq(AfterSaleCaseEntity::getCaseNo, caseNo));
                if (caseEntity == null) {
                    continue;
                }
                Map<String, Object> extra = new HashMap<>();
                extra.put("shipping_method", caseEntity.getReturnShippingMethod());
                extra.put("deadline_at",
                        caseEntity.getAutoConfirmAt() != null
                                ? caseEntity.getAutoConfirmAt().toString() : null);
                // 例外 B：先 publish，成功才 stamp
                boolean published = eventPublisher.tryPublishCaseTransitionWithContext(
                        AfterSalesEventType.CASE_RETURN_DELIVERED_24H.value(),
                        MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED_24H,
                        caseNo, caseEntity.getOrderNo(),
                        caseEntity.getBuyerId(), caseEntity.getStoreId(),
                        "system", extra);
                if (!published) {
                    log.warn("ReturnTwentyFourHourWarnScheduler.confirmWarn.publishFailed",
                            "publish 失敗，不 stamp，下一輪重發, caseNo=" + caseNo);
                    continue;
                }
                caseMapper.stampConfirmReceipt24hWarned(caseNo, DateTimeUtils.nowUtc());
                sent++;
            } catch (Exception e) {
                log.error("ReturnTwentyFourHourWarnScheduler.confirmWarn.error",
                        "處理確認收貨 24h 預警異常, caseNo=" + caseNo, e);
            }
        }

        log.info("ReturnTwentyFourHourWarnScheduler.confirmWarn.done", "送出 FCM 數=" + sent);
    }

    /**
     * §5.3.10.3 待寄回 D-1 預警掃描分支（2026-06-15 新增）。
     * 條件：status='20' AND warned_ship_back_24h_at IS NULL AND ship_back_deadline_at 在 (now, now+24h]。
     * 推送目標 = 買家（與 26/27 分支推給賣家不同）。
     */
    private void scanShipBackWarn(LocalDateTime now, LocalDateTime cutoff) {
        List<String> candidates = caseMapper.findShipBack24hWarnCandidates(now, cutoff, BATCH_LIMIT);
        if (candidates.isEmpty()) return;

        log.info("ReturnTwentyFourHourWarnScheduler.shipBackWarn.found",
                "發現 " + candidates.size() + " 筆 待寄回剩 24h 預警候選");

        int sent = 0;
        for (String caseNo : candidates) {
            try {
                AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                        new LambdaQueryWrapper<AfterSaleCaseEntity>()
                                .eq(AfterSaleCaseEntity::getCaseNo, caseNo));
                if (caseEntity == null) {
                    continue;
                }
                Map<String, Object> extra = new HashMap<>();
                extra.put("shipping_method", caseEntity.getReturnShippingMethod());
                extra.put("deadline_at",
                        caseEntity.getShipBackDeadlineAt() != null
                                ? caseEntity.getShipBackDeadlineAt().toString() : null);
                // 例外 B：先 publish，成功才 stamp
                boolean published = eventPublisher.tryPublishCaseTransitionWithContext(
                        AfterSalesEventType.CASE_RETURN_PENDING_24H.value(),
                        MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_PENDING_24H,
                        caseNo, caseEntity.getOrderNo(),
                        caseEntity.getBuyerId(), caseEntity.getStoreId(),
                        "system", extra);
                if (!published) {
                    log.warn("ReturnTwentyFourHourWarnScheduler.shipBackWarn.publishFailed",
                            "publish 失敗，不 stamp，下一輪重發, caseNo=" + caseNo);
                    continue;
                }
                caseMapper.stampShipBack24hWarned(caseNo, DateTimeUtils.nowUtc());
                sent++;
            } catch (Exception e) {
                log.error("ReturnTwentyFourHourWarnScheduler.shipBackWarn.error",
                        "處理待寄回 24h 預警異常, caseNo=" + caseNo, e);
            }
        }

        log.info("ReturnTwentyFourHourWarnScheduler.shipBackWarn.done", "送出 FCM 數=" + sent);
    }
}
