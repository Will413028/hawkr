package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.util.List;

/**
 * 退款同步 Worker。
 * 每小時（:30）檢查已完成但可能遺漏 MQ 通知的退款任務，重新發佈事件。
 *
 * <p><b>搶佔語意（issue #2294 L5）</b>：原本<b>完全沒有搶佔</b>（純 {@code selectList} → publish），
 * prod {@code replicas=2} 下同一則補發事件會被兩個 pod 各發一次。修法＝
 * <b>lease on {@code refund_tasks.resync_at}</b>（租約 3 小時 = 排程週期 × 3），
 * claim 成功才 publish。候選述詞無時間上界 ⇒ 候選窗口寬度 ∞ ⇒ 任何租約長度都合格。
 *
 * <p>🚨 <b>相鄰缺陷（本 issue 不解，但必須寫明，否則會以為加了租約就修好了）</b>：候選述詞是
 * {@code status=DONE AND updated_at < NOW()-1h}，而 {@code DONE} 是<b>終態</b>——沒有任何後續寫入
 * 會把它推出候選集合。加租約只是把「每小時重發一次」變成「每 3 小時重發一次」，<b>沒有終止條件</b>：
 * 歷史上每一筆退款成功的 task 都會被無限期週期性重發（{@code LIMIT 50} 是唯一節流），
 * 真正遺失的那一筆的補發延遲 ≈ (歷史 DONE 筆數 ÷ 50) × 3h，隨單量線性成長。
 * 終局形態（二選一）見 09 §8.5：① 補發成功後標終態欄讓 DONE 真正離開候選 ② 改由通用事件 outbox 承接。
 * 重複投遞現在可接受，是因為 order 側 {@code financial_log (type, external_txn_id)} UNIQUE
 * 會把重複判成 skip ⇒ 是噪音不是金額重複累加。
 */
@Component
public class RefundSyncWorker {

    private static final ObsLogger log = ObsLogger.forClass(RefundSyncWorker.class);

    /** issue #2294 L5：租約長度（小時）＝ 排程週期（1 小時）× 3 */
    private static final long RESYNC_LEASE_HOURS = 3L;

    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleCaseMapper caseMapper;
    private final AfterSalesEventPublisherImpl eventPublisher;

    public RefundSyncWorker(RefundTaskMapper refundTaskMapper,
                             AfterSaleCaseMapper caseMapper,
                             AfterSalesEventPublisherImpl eventPublisher) {
        this.refundTaskMapper = refundTaskMapper;
        this.caseMapper = caseMapper;
        this.eventPublisher = eventPublisher;
    }

    @Scheduled(cron = "0 30 * * * ?")
    public void syncRefundEvents() {
        // 查詢已完成但超過 1 小時的退款任務（可能 MQ 發佈失敗）
        // ⚠️ R4-P1-2（2026-08-08）：這個 now 只用於「候選述詞」與「租約未到期判定」。
        // 租約的**基準**不在這裡取（見下方迴圈內），否則整批共用同一個到期時間。
        var now = DateTimeUtils.nowUtc();
        var cutoff = now.minusHours(1);

        List<RefundTaskEntity> tasks = refundTaskMapper.selectList(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.DONE.value())
                        .lt(RefundTaskEntity::getUpdatedAt, cutoff)
                        // issue #2294 L5：租約未到期的列由別的 pod 持有，本輪不撈
                        .and(lease -> lease
                                .isNull(RefundTaskEntity::getResyncAt)
                                .or()
                                .le(RefundTaskEntity::getResyncAt, now))
                        .last("LIMIT 50")
        );

        if (tasks.isEmpty()) {
            return;
        }

        log.info("RefundSyncWorker.found",
                "發現 " + tasks.size() + " 筆可能遺漏通知的退款任務");

        for (RefundTaskEntity task : tasks) {
            try {
                // ⚠️ R4-P1-2（2026-08-08）：租約基準取 **claim 當下**而非 tick 起點的 now。
                // 取 tick 起點 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的**有效**租約
                // = RESYNC_LEASE_HOURS − (前 k−1 筆的耗時)，不變式退化成
                // 「批次總耗時 < 租約」（會隨 LIMIT 50 這個 batchSize 一起破）。
                // 改在這裡取之後，不變式降為「**單筆**最壞耗時 < 租約」——與 batchSize 無關，
                // 是結構保證而非參數巧合。述詞用的 now/cutoff 維持 tick 起點：那個方向讓
                // 候選集合與可搶佔集合更小，是保守的。
                var leaseUntil = DateTimeUtils.nowUtc().plusHours(RESYNC_LEASE_HOURS);
                // issue #2294 L5：lease claim —— 搶到才 publish（0 = 另一個 pod 已搶走）
                int claimed = refundTaskMapper.claimForResync(
                        task.getId(), WorkerTaskStatus.DONE.value(), now, leaseUntil);
                if (claimed == 0) {
                    log.info("RefundSyncWorker.claim_skip",
                            "租約由他人持有，skip, refundTaskId=" + task.getId());
                    continue;
                }

                if (task.getCaseNo() != null) {
                    // 售後案件退款完成事件；outcome 從 task 取出（case_no path 必填）
                    AfterSalePlatformDecisionOutcome outcome = AfterSalePlatformDecisionOutcome.fromValue(
                            task.getRefundOutcome());
                    // #1226：補發賣家工作區 realtime 需 storeId，task 不帶，由 caseNo 反查
                    Long storeId = caseMapper.findStoreIdByCaseNo(task.getCaseNo());
                    Long buyerId = caseMapper.findBuyerIdByCaseNo(task.getCaseNo());
                    eventPublisher.publishRefundCompleted(
                            task.getCaseNo(),
                            task.getOrderNo(),
                            storeId,
                            buyerId,
                            outcome,
                            task.getRefundAmount(),
                            task.getPaymentMethod(),
                            task.getExternalRefId()
                    );
                } else if (task.getCancelId() != null) {
                    // 賣家取消完成事件
                    eventPublisher.publishCancelCompleted(
                            task.getOrderNo(),
                            task.getCancelId()
                    );
                }

                log.info("RefundSyncWorker.republished",
                        "重新發佈退款事件, refundTaskId=" + task.getId(), "orderNo", task.getOrderNo());
            } catch (Exception e) {
                log.error("RefundSyncWorker.error",
                        "重新發佈退款事件失敗, refundTaskId=" + task.getId(), e);
            }
        }
    }
}
