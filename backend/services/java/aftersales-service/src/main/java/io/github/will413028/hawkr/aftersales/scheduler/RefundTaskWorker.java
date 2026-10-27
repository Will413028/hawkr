package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.LambdaUpdateWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleEvidenceType;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 退款任務 Worker。
 * 定期輪詢待執行/失敗的退款任務，呼叫 Payment Service 執行退款。
 *
 * <p><b>搶佔語意（issue #2294 C1）</b>：claim 是 <b>lease on {@code next_retry_time}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒，長度推導見該常數），不是 {@code SET status=PROCESSING} 的永久旗標。
 * {@code status} 保留但角色降為
 * 「可觀測資訊」（讓運維在 DB 上讀出「這筆正在被處理」）。候選述詞同步涵蓋 {@code PROCESSING}，
 * 否則租約到期的列回不來——那只是把永久旗標寫得更像 lease。零 migration（欄位與索引皆既有）。
 *
 * <p><b>與冪等鍵的分工</b>：本 worker 送給 {@code CreateRefund} 的冪等鍵是
 * <b>{@code refund_no = case_no + "-" + retry_count}</b>（{@code cancel_id} 路徑同理，見
 * {@link #processTask} 組裝處），<b>不是</b>單純的 {@code case_no}。這一點是理解整條防線的關鍵：
 * <ul>
 *   <li><b>租約到期被另一個 pod 重搶</b> ⇒ {@code retry_count} <b>未變</b> ⇒ <b>同一把鍵</b>
 *       ⇒ payment 側 {@code idx_payment_refunds_idempotency_key} 命中，回傳既有退款、不重打 PayUni。
 *       claim 是第一道防線（少打一次），冪等鍵是最終防線（真打了兩次也不會退兩次）。</li>
 *   <li><b>走失敗路徑重試</b>（{@link #handleRefundFailure}）⇒ {@code retry_count++}
 *       ⇒ <b>換一把鍵</b> ⇒ 冪等鍵這道防線<b>刻意不生效</b>（那正是「重試」的定義）。
 *       此時擋在前面的是 payment 側與冪等鍵無關的兩道金額防線——完整推導見
 *       {@link PaymentGrpcClient#CREATE_REFUND_DEADLINE_SEC}，
 *       它就是建立在「含 {@code retryCount} ⇒ 換鍵」這個事實上。</li>
 * </ul>
 */
@Component
public class RefundTaskWorker {

    private static final ObsLogger log = ObsLogger.forClass(RefundTaskWorker.class);

    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleCaseMapper caseMapper;
    private final SellerCancelRequestMapper cancelMapper;
    private final PaymentGrpcClient paymentGrpcClient;
    private final AfterSaleAuditService auditService;
    private final AfterSalesEventPublisherImpl eventPublisher;

    public RefundTaskWorker(RefundTaskMapper refundTaskMapper,
                             AfterSaleCaseMapper caseMapper,
                             SellerCancelRequestMapper cancelMapper,
                             PaymentGrpcClient paymentGrpcClient,
                             AfterSaleAuditService auditService,
                             AfterSalesEventPublisherImpl eventPublisher) {
        this.refundTaskMapper = refundTaskMapper;
        this.caseMapper = caseMapper;
        this.cancelMapper = cancelMapper;
        this.paymentGrpcClient = paymentGrpcClient;
        this.auditService = auditService;
        this.eventPublisher = eventPublisher;
    }

    /** 信用卡路徑重試上限（retry_count 達此值後不再重試，直接終止為 FAILED 進異常清單） */
    private static final int CC_RETRY_LIMIT = 1;

    /** 信用卡失敗後的固定重試延遲（秒） */
    private static final long CC_RETRY_DELAY_SEC = 30L;

    /**
     * issue #2294 C1：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：候選述詞無時間上界 ⇒ 候選窗口寬度 <b>∞</b>
     * ⇒ 任何租約長度都合格。cron 週期（tick 10 秒）遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步：租約長度必須 &gt; 單筆工作的最壞耗時）——issue #2294 P1-1 補齊：
     * claim 之後、下一次狀態寫入之前，唯一的長工作是
     * {@link PaymentGrpcClient#createRefund}，其最壞耗時由
     * {@link PaymentGrpcClient#CREATE_REFUND_DEADLINE_SEC}（100 秒）硬性封頂
     * （在此之前該呼叫<b>沒有任何 deadline</b>，下界根本無從建立，本常數的舊值 30 秒只論證了上界）。
     * 取 <b>120 秒 = 100 秒 deadline ＋ 20 秒</b>，餘裕留給 claim→呼叫之間與失敗路徑的 DB 寫入。
     * 不變式 {@code CLAIM_LEASE_SEC > PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC} 由
     * {@code RefundTaskWorkerClaimTest} 機械斷言。
     *
     * <p>⚠️ <b>加長租約不會拖慢正常重試</b>：失敗路徑寫的
     * {@code next_retry_time = now + CC_RETRY_DELAY_SEC(30s)} 與租約<b>寫同一欄</b>且在 claim 之後執行，
     * 天然覆蓋租約值（設計如此）。租約只影響「pod 在 claim 與退款之間崩潰」的救援延遲
     * （由 30 秒延長為 120 秒，屬設計書 09 §8.5 已載明的可接受退化）。
     */
    static final long CLAIM_LEASE_SEC = 120L;

    @Scheduled(fixedDelayString = "${aftersales.refund.worker-interval-ms:10000}")
    public void processRefundTasks() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        // 僅處理非 ATM 路徑；ATM 退款由 RefundResultConsumer 依 webhook 推進
        // 掃描條件（issue #2294 C1：claim 從「換狀態」改成「推時間」後的候選述詞）：
        //   PENDING   且 租約未生效或已到期 → 撈
        //   FAILED    且 next_retry_time 已到 → 撈（retry 中）
        //   FAILED    且 next_retry_time=NULL → 不撈（**終態**；避免無限 loop 再處理已升級異常的 task）
        //   PROCESSING 且 租約已到期 → 撈（pod 在 claim 與 CreateRefund 之間崩潰的救援路徑；
        //              舊實作沒有這條，該筆退款永久卡在 PROCESSING）
        // ⚠️ PENDING 分支原本**沒有時間條件**，光推 next_retry_time 不會讓它退出候選 ⇒ 必須補。
        // ⚠️ FAILED 的 isNotNull 是**終態守門**，不可為了「三個分支寫成一個共同述詞」而拿掉。
        List<RefundTaskEntity> tasks = refundTaskMapper.selectList(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .ne(RefundTaskEntity::getPaymentMethod, PaymentMethod.ATM.value())
                        .and(outer -> outer
                                .nested(pending -> pending
                                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.PENDING.value())
                                        .and(lease -> lease
                                                .isNull(RefundTaskEntity::getNextRetryTime)
                                                .or()
                                                .le(RefundTaskEntity::getNextRetryTime, now)))
                                .or(failed -> failed
                                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.FAILED.value())
                                        .isNotNull(RefundTaskEntity::getNextRetryTime)
                                        .le(RefundTaskEntity::getNextRetryTime, now))
                                .or(processing -> processing
                                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.PROCESSING.value())
                                        .and(lease -> lease
                                                .isNull(RefundTaskEntity::getNextRetryTime)
                                                .or()
                                                .le(RefundTaskEntity::getNextRetryTime, now))))
                        .last("LIMIT 10")
        );

        for (RefundTaskEntity task : tasks) {
            try {
                processTask(task);
            } catch (Exception e) {
                log.error("RefundTaskWorker.processTask.error",
                        "處理退款任務異常, refundTaskId=" + task.getId(), e);
            }
        }
    }

    private void processTask(RefundTaskEntity task) {
        Long taskId = task.getId();
        LocalDateTime now = DateTimeUtils.nowUtc();

        // issue #2294 C1：claim 從「換狀態」改成「**推時間**」（lease）。
        // 舊 claim 只 SET status=PROCESSING，而 PROCESSING 不在候選集合、沒有任何時間欄能讓它回來
        //  ⇒ pod 在 claim 與 CreateRefund 之間崩潰（進程消失，catch 不會跑）＝該筆退款永久卡死。
        // 新 claim 同時推 next_retry_time = now + 租約：status 保留但角色降為「可觀測資訊」，
        // 租約到期後該列會被候選述詞重新撈回。
        // 失敗路徑寫的 next_retry_time = now+30s 在 claim 之後執行，天然覆蓋租約值（設計如此）。
        int claimed = refundTaskMapper.update(null,
                new LambdaUpdateWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getId, taskId)
                        .and(status -> status
                                .in(RefundTaskEntity::getStatus,
                                        WorkerTaskStatus.PENDING.value(),
                                        WorkerTaskStatus.PROCESSING.value())
                                .or(failed -> failed
                                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.FAILED.value())
                                        .isNotNull(RefundTaskEntity::getNextRetryTime)))
                        .and(lease -> lease
                                .isNull(RefundTaskEntity::getNextRetryTime)
                                .or()
                                .le(RefundTaskEntity::getNextRetryTime, now))
                        .set(RefundTaskEntity::getStatus, WorkerTaskStatus.PROCESSING.value())
                        .set(RefundTaskEntity::getNextRetryTime, now.plusSeconds(CLAIM_LEASE_SEC))
                        .set(RefundTaskEntity::getUpdatedAt, now)
        );
        if (claimed == 0) {
            // 已被其他 Worker 搶走（租約未到期），跳過
            return;
        }

        // 重新查詢最新狀態
        task = refundTaskMapper.selectById(taskId);

        // 組裝退款單號（用案件編號或取消 ID 作為退款識別）
        String refundNo;
        if (task.getCaseNo() != null) {
            refundNo = task.getCaseNo() + "-" + task.getRetryCount();
        } else {
            refundNo = "C" + task.getCancelId() + "-" + task.getRetryCount();
        }

        // §2A.1.4：零元訂單沒有退款概念，跳過 payment-service createRefund 呼叫，直接走成功路徑。
        // 對 aftersales case state machine 透明（等同「refund step 即時完成」），下游 refund.completed
        // event 仍正常發出（券恢復、庫存恢復、訂單狀態更新一致）。
        if (PaymentMethod.ZERO_AMOUNT.value().equals(task.getPaymentMethod())) {
            log.info("RefundTaskWorker.zeroAmount.skip",
                    "零元訂單退款 step 自動完成（不呼叫 payment-service），refundTaskId=" + taskId);
            markTaskDoneAndAdvance(task, refundNo);
            return;
        }

        // 呼叫 Payment Service 執行退款
        try {
            CreateRefundResponse resp = paymentGrpcClient.createRefund(
                    task.getTransactionId(),
                    ProtoConversionUtils.formatLong(task.getRefundAmount()),
                    "售後退款",
                    refundNo
            );

            if (resp.getCode() == 0) {
                // 退款成功（或已受理）
                markTaskDoneAndAdvance(task, refundNo);
            } else {
                // 退款失敗：持久化 PayUni 原始 status/message（last_error 帶 payment 回報原話，非僅 code）
                handleRefundFailure(task, refundNo,
                        "Payment 回報錯誤: code=" + resp.getCode() + ", msg=" + resp.getMessage(),
                        resp.getData().getProviderStatus(), resp.getData().getProviderMessage());
            }
        } catch (Exception e) {
            handleRefundFailure(task, refundNo, "呼叫 Payment Service 異常: " + e.getMessage(), null, null);
        }
    }

    /**
     * 標 task DONE + 推進上層狀態 + 發 refund.completed event + audit。
     * 共用路徑：信用卡同步成功、ZERO_AMOUNT 短路（§2A.1.4）。
     */
    private void markTaskDoneAndAdvance(RefundTaskEntity task, String refundNo) {
        Long taskId = task.getId();
        refundTaskMapper.update(null,
                new LambdaUpdateWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getId, taskId)
                        .set(RefundTaskEntity::getStatus, WorkerTaskStatus.DONE.value())
                        .set(RefundTaskEntity::getExternalRefId, refundNo)
                        .set(RefundTaskEntity::getUpdatedAt, DateTimeUtils.nowUtc()));
        task.setStatus(WorkerTaskStatus.DONE.value());
        task.setExternalRefId(refundNo);

        closeParentEntity(task);
        publishRefundCompletedEvent(task, refundNo);

        auditService.log(task.getCaseNo(), task.getCancelId(), null, AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.REFUND_SUCCEEDED.value(),
                Map.of("refundTaskId", taskId, "refundNo", refundNo));

        log.info("RefundTaskWorker.success",
                "退款成功, refundTaskId=" + taskId, "refundNo", refundNo);
    }

    /**
     * 信用卡退款失敗處理：
     * retry_count < CC_RETRY_LIMIT(1) → retry_count++，固定 30s 後重試，狀態 FAILED（等 Worker 下輪撈起）
     * retry_count >= CC_RETRY_LIMIT    → 終止為 FAILED（next_retry_time=null，不再重試），進 admin 異常清單
     */
    private void handleRefundFailure(RefundTaskEntity task, String refundNo, String errorMsg,
                                     String payuniStatus, String payuniMessage) {
        int currentRetry = task.getRetryCount();
        Long taskId = task.getId();
        LocalDateTime now = DateTimeUtils.nowUtc();

        if (currentRetry < CC_RETRY_LIMIT) {
            int newRetryCount = currentRetry + 1;
            LocalDateTime nextRetry = now.plusSeconds(CC_RETRY_DELAY_SEC);
            refundTaskMapper.update(null,
                    new LambdaUpdateWrapper<RefundTaskEntity>()
                            .eq(RefundTaskEntity::getId, taskId)
                            .set(RefundTaskEntity::getStatus, WorkerTaskStatus.FAILED.value())
                            .set(RefundTaskEntity::getExternalRefId, refundNo)
                            .set(RefundTaskEntity::getLastError, errorMsg)
                            // 持久化 PayUni 原始循線（payment 回非 0 時帶；例外路徑為 null 不覆蓋既有值）
                            .set(payuniStatus != null, RefundTaskEntity::getPayuniStatus, payuniStatus)
                            .set(payuniMessage != null, RefundTaskEntity::getPayuniMessage, payuniMessage)
                            .set(RefundTaskEntity::getRetryCount, newRetryCount)
                            .set(RefundTaskEntity::getNextRetryTime, nextRetry)
                            .set(RefundTaskEntity::getUpdatedAt, now));

            auditService.log(task.getCaseNo(), task.getCancelId(), null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.REFUND_FAILED.value(),
                    Map.of("refundTaskId", taskId, "retryCount", newRetryCount,
                            "nextRetryTime", nextRetry.toString(), "error", errorMsg,
                            "finalFailed", false));

            log.warn("RefundTaskWorker.failed.willRetry",
                    "退款失敗，排隊重試, retryCount=" + newRetryCount + ", nextRetry=" + nextRetry,
                    "refundTaskId", String.valueOf(taskId));
        } else {
            // 終態：next_retry_time=NULL 確保 Worker 下輪不再撈此 task
            refundTaskMapper.update(null,
                    new LambdaUpdateWrapper<RefundTaskEntity>()
                            .eq(RefundTaskEntity::getId, taskId)
                            .set(RefundTaskEntity::getStatus, WorkerTaskStatus.FAILED.value())
                            .set(RefundTaskEntity::getExternalRefId, refundNo)
                            .set(RefundTaskEntity::getLastError, errorMsg)
                            // 持久化 PayUni 原始循線（payment 回非 0 時帶；例外路徑為 null 不覆蓋既有值）
                            .set(payuniStatus != null, RefundTaskEntity::getPayuniStatus, payuniStatus)
                            .set(payuniMessage != null, RefundTaskEntity::getPayuniMessage, payuniMessage)
                            .set(RefundTaskEntity::getNextRetryTime, null)
                            .set(RefundTaskEntity::getUpdatedAt, now));

            auditService.log(task.getCaseNo(), task.getCancelId(), null, AfterSaleAuditActor.SYSTEM.value(),
                    AfterSaleAuditAction.REFUND_FAILED.value(),
                    Map.of("refundTaskId", taskId, "retryCount", currentRetry,
                            "error", errorMsg, "finalFailed", true));

            log.warn("RefundTaskWorker.failed.finalFailed",
                    "退款最終失敗（超過 retry 上限），進異常清單, retryCount=" + currentRetry,
                    "refundTaskId", String.valueOf(taskId));
        }
    }

    /**
     * 退款成功後關閉上層實體（售後案件 → CLOSED / 取消請求 → COMPLETED）。
     * 09 §2.1 / §2.3 INV-3 / INV-4：走 mapper 自訂 SQL（與 RefundResultConsumer.closeParentAndPublish
     * 共用規格），CAS WHERE status IN ('40' / '10') 防 webhook / 兩 worker 並發 race。
     * rows=0 視為「已關閉 / 已完成 / 前置不合法」冪等 skip。
     */
    private void closeParentEntity(RefundTaskEntity task) {
        if (task.getCaseNo() != null) {
            caseMapper.closeIfNotClosed(task.getCaseNo(), DateTimeUtils.nowUtc());
        } else if (task.getCancelId() != null) {
            cancelMapper.completeIfNotCompleted(task.getCancelId());
        }
    }

    /**
     * 發佈退款完成 MQ 事件。
     * case_no path：outcome 從 task 取出，case_no path 必填（由 V3 CHECK constraint 保證；
     * 缺值由 publisher fail-loud 阻斷）。
     * cancel_id path：走 cancel.completed，下游不需 outcome。
     */
    private void publishRefundCompletedEvent(RefundTaskEntity task, String refundNo) {
        if (task.getCaseNo() != null) {
            AfterSalePlatformDecisionOutcome outcome = AfterSalePlatformDecisionOutcome.fromValue(
                    task.getRefundOutcome());
            // #1226：補發賣家工作區 realtime 需 storeId，task 不帶，由 caseNo 反查
            Long storeId = caseMapper.findStoreIdByCaseNo(task.getCaseNo());
            Long buyerId = caseMapper.findBuyerIdByCaseNo(task.getCaseNo());
            eventPublisher.publishRefundCompleted(
                    task.getCaseNo(), task.getOrderNo(), storeId, buyerId,
                    outcome, task.getRefundAmount(),
                    task.getPaymentMethod(), refundNo);
        } else if (task.getCancelId() != null) {
            eventPublisher.publishCancelCompleted(task.getOrderNo(), task.getCancelId());
        }
    }
}
