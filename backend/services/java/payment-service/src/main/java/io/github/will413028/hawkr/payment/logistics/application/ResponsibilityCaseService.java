package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxPublisher;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityImpactPreview;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityImpactPreviewCodec;
import io.github.will413028.hawkr.payment.logistics.mq.payload.FulfillmentResponsibilityFinalizedPayload;
import io.github.will413028.hawkr.payment.logistics.mq.payload.FulfillmentResponsibilityUnresolvedPayload;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentCaseResolutionRepository;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Objects;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/**
 * 責任判定案件的核心 primitive（WP2A）。
 *
 * <p><b>本類別提供什麼</b>：建案（N17）、提案、定案（N16）、駁回四個 application 方法。
 * <b>不提供什麼</b>：Admin gRPC／Gateway 端點、maker-checker HTTP 契約、ADM UI——那些是 WP2B。
 * 本類別的方法就是 WP2B 要呼叫的東西。
 *
 * <p>🔴 <b>{@code fullRefundBasis} 的取得契約（Phase A / Phase B）</b>
 * ——{@link #propose} 與 {@link #finalizeResponsibility} 都收這個參數：
 * <ol>
 *   <li><b>Phase A（交易外）</b>：呼叫端經 {@code OrderGrpcClient.getOrderForPayment(orderNo)}
 *       取得 {@code orders.total_actual_amount}。<b>必須在交易外</b>——36 §3.3.1 明令
 *       業務交易內不得做 gRPC（持有列鎖跨越網路呼叫會把鎖的持有時間綁在對端的延遲上）。</li>
 *   <li><b>Phase B（交易內）</b>：本類別以它 ＋ 持久化的成因算出／重算預覽。</li>
 * </ol>
 * <p>🔴 <b>這個兩段式沒有 TOCTOU 窗口，而那是被證明出來的不是假設的</b>：
 * {@code orders.total_actual_amount} 建單 INSERT 之後<b>永不改變</b>——
 * 生產碼只有三個 INSERT-time setter（全在 {@code PENDING_PAYMENT}）、
 * {@code OrderRepository} 24 個 {@code @Update} 的 SET 欄位聯集不含它、
 * {@code updateById} 零呼叫點，且 {@code OrderCancelTimestampSqlTest} 在 CI 把
 * {@code orders} 的寫入面鎖死在單一檔案。退款走<b>獨立欄位</b>
 * {@code refund_total_amount}，淨額查詢時才算 ⇒ 退款不回寫訂單金額。
 * <p>⚠️ 同形先例：{@code SellerCancelRefundTxHandler} 今天就在真實金流上跑
 * 「交易外讀 → 交易內重讀同一個值」，其正確性依賴的是同一個不變式。
 * <p>🔴 <b>WP2B 不得讓 ADM client 提供這個值</b>：它是 server 從 order-service 讀到的事實，
 * 不是使用者輸入。與 {@code sourceService} 同一個 trust boundary。
 *
 * <p>🔴 <b>四個 primitive 的不變式（65 §5.5.2 E0／E0.1／E0.2、36 §3.3）</b>：
 * <ol>
 *   <li><b>同一 fulfillment leg 終其一生只有一張責任案件</b>——由 {@code uq_exc_responsibility_subject}
 *       承擔，<b>不靠鎖</b>（鎖的範圍是同一張 case，跨 subject 本來就該並行）。</li>
 *   <li><b>提案與定案都先 {@code SELECT … FOR UPDATE} 鎖同一張 case</b>。明令廢除
 *       snapshot-based finality：「掃一次、看到沒有 AWAITING_REVIEW」是 read-then-act，
 *       兩個並行交易會同時讀到「沒有」然後同時往下走，而且不會有任何錯誤訊息。</li>
 *   <li><b>只有兩道 CAS 都 rows=1 的那一次交易才 INSERT N16</b>——這就是 producer 側的 L2 冪等，
 *       不需要任何去重鍵（{@code outbox_events} 也沒有）。</li>
 *   <li><b>駁回不動案件</b>：resolution → REJECTED，案件維持 {@code IN_PROGRESS}、owner 保留，
 *       同一張 case 可以再提新案。🔴 <b>不得</b>把駁回做成 case DISMISSED——那會讓案件
 *       永遠到不了終態，而 Order 正卡在 35 等它。</li>
 * </ol>
 *
 * <p>🔴 <b>本類別不做 downstream 副作用</b>：不推 Order 35→54、不建撥款任務、不退款、
 * 不建 logistics receivable、不記 KPI。那些是 N16 的 consumer（WP3／WP4）在<b>各自的交易</b>裡做。
 * 兩個 transaction boundary 維持分離，不得合併成 distributed transaction。
 */
@Service
public class ResponsibilityCaseService {

    private static final ObsLogger log = ObsLogger.forClass(ResponsibilityCaseService.class);

    /** 36 §7.4 envelope：ISO 8601 UTC 帶 Z。 */
    private static final DateTimeFormatter ISO_UTC = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'");

    private static final String EVENT_VERSION = "v1";
    private static final String SEVERITY_P1 = "P1";

    private final FulfillmentExceptionCaseRepository cases;
    private final FulfillmentCaseResolutionRepository resolutions;
    private final PaymentOutboxWriter outboxWriter;
    private final PaymentOutboxPublisher outboxPublisher;
    private final ObjectMapper objectMapper;
    private final Supplier<Long> ids;
    private final Supplier<LocalDateTime> nowUtc;

    @Autowired
    public ResponsibilityCaseService(FulfillmentExceptionCaseRepository cases,
                                     FulfillmentCaseResolutionRepository resolutions,
                                     PaymentOutboxWriter outboxWriter,
                                     PaymentOutboxPublisher outboxPublisher,
                                     ObjectMapper objectMapper) {
        this(cases, resolutions, outboxWriter, outboxPublisher, objectMapper,
                TsidUtil::generate, DateTimeUtils::nowUtc);
    }

    /** 測試用：注入 id 產生器與時鐘（10 檔 §21：時間敏感邏輯不得靠 sleep 或本機 timezone）。 */
    public ResponsibilityCaseService(FulfillmentExceptionCaseRepository cases,
                                     FulfillmentCaseResolutionRepository resolutions,
                                     PaymentOutboxWriter outboxWriter,
                                     PaymentOutboxPublisher outboxPublisher,
                                     ObjectMapper objectMapper,
                                     Supplier<Long> ids,
                                     Supplier<LocalDateTime> nowUtc) {
        this.cases = cases;
        this.resolutions = resolutions;
        this.outboxWriter = outboxWriter;
        this.outboxPublisher = outboxPublisher;
        this.objectMapper = objectMapper;
        this.ids = ids;
        this.nowUtc = nowUtc;
    }

    // =========================================================================
    // Commands / Results
    // =========================================================================

    /**
     * 建立責任判定案件。
     *
     * @param subjectRef      🔴 provider 配送 <b>leg</b> 識別（{@code provider_delivery_ref}），
     *                        不是 order_no、不是 shipment PK——同一張訂單的正逆向兩條 leg
     *                        各自有獨立的責任判定
     * @param sourceService   🔴 <b>這個例外狀況的「來源服務」，不是「哪個服務擁有這張表」</b>
     *                        ——兩張表由 payment-service 擁有，但觸發來源可能是 {@code shipping}
     *                        （provider 終局處置格判不出成因）或 {@code order}。
     *                        由呼叫端依真正的 trigger 決定；合法值域與 trust boundary 見
     *                        {@link ResponsibilityCaseRules#SOURCE_SERVICES}
     * @param occurredAt      🔴 來源端的業務發生時間，不得用 {@code NOW()} 冒充
     */
    public record OpenUnresolvedCommand(String subjectRef,
                                        String sourceService,
                                        Long storeId,
                                        String orderNo,
                                        Long shipmentId,
                                        String shippingMethod,
                                        String detailJson,
                                        LocalDateTime occurredAt) {
    }

    /**
     * 建案結果。
     *
     * @param created {@code true} = 這次真的開了（<b>且只有這一次</b>發了 N17）；
     *                {@code false} = 該 leg 已有責任案件 ⇒ 沒有發第二筆事件
     */
    public record OpenResult(long caseId, String caseNo, boolean created) {
    }

    /**
     * Maker 的提案。
     *
     * <p>🔴 {@code proposedResponsibilityCause} 是<b>提案的標的物</b>（V13）：
     * 覆核者能做的只有 APPROVE／REJECT 這個成因，他<b>提供不了</b>另一個。
     * 責任判定案件必填四個 resolved 成因之一；其他 category 的提案必須為 {@code null}。
     */
    public record ProposeCommand(long caseId,
                                 String resolutionType,
                                 String proposedResponsibilityCause,
                                 String reason,
                                 long proposedByAccountId) {
    }

    /**
     * Checker 的覆核決定。
     *
     * <p>🔴 <b>刻意只有四個欄位</b>（65 §6.4.2 A-5 ＝ {@code decision} ＋ {@code comment}）。
     * 覆核者能表達的只有「批准或駁回<b>這一筆提案</b>」，其餘一切都是 server 端事實：
     *
     * <table border="1">
     *   <caption>被移除的欄位與它們現在的來源</caption>
     *   <tr><th>欄位</th><th>現在從哪來</th><th>為什麼不能由 checker 給</th></tr>
     *   <tr><td>{@code responsibilityCause}</td>
     *       <td>{@code fulfillment_case_resolutions.proposed_responsibility_cause}（V13）</td>
     *       <td>它是 maker 提案的標的物；由 checker 給等於雙人制只剩兩個簽名</td></tr>
     *   <tr><td>{@code orderNo}／{@code shipmentId}／{@code shippingMethod}</td>
     *       <td>{@code fulfillment_exception_cases} 建案當下凍結的那一列（V14）</td>
     *       <td>由呼叫端再傳一次 ⇒ N17 與 N16 可以帶不同的 order_no，而那是
     *           order-service {@code 35→54} CAS 的錨，對不上時全鏈路無訊號</td></tr>
     *   <tr><td>{@code providerDeliveryRef}</td><td>{@code case.subject_ref}</td>
     *       <td>同上，本來就已經取自鎖住的那一列</td></tr>
     *   <tr><td>{@code sourceService}</td><td>建案時決定（R1 的 allowlist ＋ fail-fast）</td>
     *       <td>它是 backend internal metadata，不是使用者輸入</td></tr>
     * </table>
     *
     * <p>🔴 <b>WP2B 不得把任何一欄加回來。</b>{@code reviewedByAccountId} 也必須從
     * {@code GrpcAuthContext.getAccountId()} 取，<b>不得</b>從 request DTO 讀。
     * 契約測試 {@code ctx3_finalizeCommandHasNoEventFacts} 以反射釘住這件事。
     */
    public record FinalizeCommand(long caseId,
                                  long resolutionId,
                                  String reviewComment,
                                  long reviewedByAccountId) {
    }

    public record RejectCommand(long caseId,
                                long resolutionId,
                                String reviewComment,
                                long reviewedByAccountId) {
    }

    /**
     * CAS 的結果。
     *
     * <p>🔴 {@link #STALE} <b>不是成功</b>：它表示「狀態已不在 allowed-from」，呼叫端
     * <b>不得</b>讀成「別人先做過了」就往下走發事件。它同時也是 retry 的正常結果
     * （已經 final 的案件再 finalize 一次 ⇒ STALE ⇒ 不發第二筆 N16）。
     */
    public enum Outcome {
        APPLIED,
        STALE,
        /**
         * 🔴 影響預覽已過期（65 §6.4.2 硬約束 2）。
         *
         * <p>提案當下算出的 {@code refundAmount} 與覆核當下重算的結果不一致
         * ⇒ <b>覆核者看到的金額與實際會執行的金額不同</b>，而那正是 maker-checker
         * 在保護的那件事本身。此時<b>不進任何 CAS、不改 resolution、不改 case、不發 N16</b>。
         *
         * <p>⚠️ WP2B 的 gRPC mapper 要把它映成
         * {@code 9575 EXCEPTION_CASE_IMPACT_PREVIEW_STALE}。
         * <p>⚠️ 它與 {@link #STALE} <b>不是同一件事</b>：{@code STALE} 是「狀態已不在
         * allowed-from」（retry 的正常結果）；本值是「狀態沒問題，但金額前提變了」。
         * 混成同一個值會讓呼叫端分不出「已經有人做過了」與「請重新產生預覽」。
         */
        STALE_PREVIEW
    }

    public record FinalizeResult(Outcome outcome, String caseNo, Long outboxId) {
    }

    // =========================================================================
    // 1. 建案 ＋ N17（同一交易）
    // =========================================================================

    /**
     * 建立責任判定案件；只有<b>真的建立成功</b>的那一次才發 N17。
     *
     * <p>🔴 <b>L2 冪等 ＝ guarded case insert 本身</b>（36 §3.3）：走的是不指名 conflict target 的
     * {@code ON CONFLICT DO NOTHING}，{@code rows=0} ⇒ 該 leg 已有案件 ⇒ <b>不發第二筆事件</b>。
     * 不得先 SELECT 再 INSERT——那是 read-then-act，兩個並行交易會同時讀到「沒有」。
     *
     * <p>⚠️ 建案時 {@code status=OPEN} ⇒ {@code owner_account_id} 必須為 {@code NULL}
     * （{@code chk_exc_owner}）。系統自動建案不指派承辦人。
     */
    @Transactional
    public OpenResult openUnresolved(OpenUnresolvedCommand cmd) {
        // 🔴 fail-closed 必須在**任何 DB 互動之前**：下一句 cases.nextCaseNo() 就會碰 DB
        //    （取 exception_case_no_seq 的 nextval）。等 DB 的 chk_exc_source 丟 23514 才擋，
        //    呼叫端拿到的是 500，而且交易已經 abort（25P02）——連帶那一列 outbox 也寫不進去。
        ResponsibilityCaseRules.assertSourceService(cmd.sourceService());
        // 🔴 同一個理由（在碰 DB 之前）：責任案件的 order_no／shipping_method 必填。
        //    等 DB 的 chk_exc_responsibility_context 丟 23514 ⇒ 呼叫端拿到 500，
        //    且交易已 abort（25P02）連帶 outbox 那一列也寫不進去。
        ResponsibilityCaseRules.assertEventContext(cmd.orderNo(), cmd.shippingMethod());

        FulfillmentExceptionCaseEntity e = new FulfillmentExceptionCaseEntity();
        e.setId(ids.get());
        e.setCaseNo(cases.nextCaseNo());
        e.setSourceService(cmd.sourceService());
        e.setCategory(ResponsibilityCaseRules.CATEGORY);
        e.setSeverity(SEVERITY_P1);
        e.setSubjectType(ResponsibilityCaseRules.SUBJECT_TYPE);
        e.setSubjectRef(cmd.subjectRef());
        e.setStoreId(cmd.storeId());
        e.setStatus(ResponsibilityCaseRules.STATUS_OPEN);
        e.setDedupKey(ResponsibilityCaseRules.dedupKey(cmd.subjectRef()));
        // 🔴 Immutable event context（V14）：建案當下凍結，N17 與 N16 都從這一列取值。
        //    在此之前這三個值只拿去組 N17 就丟掉，finalize 時由呼叫端再傳一次
        //    ⇒ 同一個 leg 的兩筆事件可以帶不同的 order_no，而 server 端零比對。
        e.setOrderNo(cmd.orderNo());
        e.setShipmentId(cmd.shipmentId());
        e.setShippingMethod(cmd.shippingMethod());
        e.setDetail(cmd.detailJson() == null ? "{}" : cmd.detailJson());
        e.setProposedActionCodes("[]");
        e.setOccurredAt(cmd.occurredAt());

        boolean created = cases.openResponsibilityCaseIfAbsent(e);
        if (!created) {
            // 回讀既有案件：ON CONFLICT DO NOTHING 不回傳衝突的那一列。
            // 🔴 這裡**不**發事件——N17 的 L2 冪等就是「只有真的寫進一列那次才發」。
            FulfillmentExceptionCaseEntity existing = cases.findResponsibilityCase(
                            ResponsibilityCaseRules.CATEGORY,
                            ResponsibilityCaseRules.SUBJECT_TYPE, cmd.subjectRef())
                    .orElseThrow(() -> new IllegalStateException(
                            "responsibility: 建案被唯一約束擋下但回讀不到既有案件"
                                    + "（subjectRef=" + cmd.subjectRef() + "）——"
                                    + "這代表擋下它的不是 lifetime UNIQUE，而是另一條約束，不得當成冪等跳過"));
            log.info("ResponsibilityCaseService.open.duplicate",
                    "該 leg 已有責任判定案件，不重複開案也不發 N17 case_no=" + existing.getCaseNo()
                            + " subject_ref=" + cmd.subjectRef());
            return new OpenResult(existing.getId(), existing.getCaseNo(), false);
        }

        FulfillmentResponsibilityUnresolvedPayload payload =
                new FulfillmentResponsibilityUnresolvedPayload(
                        UUID.randomUUID().toString(),
                        MqConstants.EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_UNRESOLVED,
                        EVENT_VERSION,
                        iso(cmd.occurredAt()),
                        currentTraceId(),
                        currentCorrelationId(),
                        // 🔴 一律取自**剛剛寫進去的那一列**，不是 request 參數。
                        //    兩者今天當然相等——重點是結構上只剩一個來源，
                        //    不會出現「DB 存一份、event 用另一份」。
                        e.getOrderNo(),
                        tsidString(e.getShipmentId()),
                        e.getShippingMethod(),
                        e.getSubjectRef(),
                        e.getCaseNo(),
                        Boolean.FALSE);

        OutboxEventEntity outbox = enqueue(
                MqConstants.EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_UNRESOLVED,
                MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED,
                payload);
        dispatchAfterCommit(outbox);

        log.info("ResponsibilityCaseService.open.created",
                "責任判定案件已建立並排入 N17 case_no=" + e.getCaseNo()
                        + " subject_ref=" + cmd.subjectRef() + " outbox_id=" + outbox.getId());
        return new OpenResult(e.getId(), e.getCaseNo(), true);
    }

    // =========================================================================
    // 2. 提案（不 finalize、不發事件）
    // =========================================================================

    /**
     * 建立責任判定提案。
     *
     * <p>🔴 <b>先鎖後判，不是先查再寫</b>（65 §5.5.2 E0.1 proposal path）。
     * <p>⚠️ 案件仍在 {@code OPEN} 時，本方法會同時把它推進 {@code IN_PROGRESS} 並指派提案者為
     * 承辦人——{@code chk_exc_owner} 要求「離開 OPEN ⇒ 必有承辦人與指派時間」，三件同一句寫。
     *
     * @return 新建的 resolution id
     * @throws IllegalStateException 案件不存在、不是責任判定案件、或已不在 allowed-from
     */
    @Transactional
    public long propose(ProposeCommand cmd, long fullRefundBasis) {
        FulfillmentExceptionCaseEntity c = lockResponsibilityCase(cmd.caseId());
        if (!ResponsibilityCaseRules.isActionable(c.getStatus())) {
            // 🔴 對一張已定案的案件提新方案是操作錯誤，不是 retry ⇒ 必須讓呼叫端知道。
            //    RESOLVED 是終局：不得 reopen、不得改判、不得建第二張案件。
            throw new IllegalStateException(
                    "responsibility: 案件已不可提案（caseId=" + cmd.caseId() + " status=" + c.getStatus()
                            + "）；RESOLVED 是終局，正確的續行方式不存在——"
                            + "責任判定一旦 final 就不再翻案");
        }
        ResponsibilityCaseRules.assertResponsibilityType(c.getCategory(), cmd.resolutionType());
        // 🔴 成因是提案的標的物，不是覆核時才補的欄位（V13）。fail-closed：提不出成因就不該提案。
        ResponsibilityCaseRules.assertProposableCause(c.getCategory(), cmd.proposedResponsibilityCause());

        if (ResponsibilityCaseRules.STATUS_OPEN.equals(c.getStatus())) {
            boolean assigned = cases.assignOwner(c.getId(), ResponsibilityCaseRules.STATUS_IN_PROGRESS,
                    cmd.proposedByAccountId(), ResponsibilityCaseRules.ALLOWED_FROM);
            if (!assigned) {
                // 🔴 我們持有這張 case 的鎖，狀態不可能被別人改掉 ⇒ 走到這裡代表不變式壞了。
                throw new IllegalStateException(
                        "responsibility: 已持有 case 鎖卻無法指派承辦人（caseId=" + c.getId()
                                + " status=" + c.getStatus() + "）——allowed-from 與鎖的語意不一致");
            }
        }

        FulfillmentCaseResolutionEntity r = new FulfillmentCaseResolutionEntity();
        r.setId(ids.get());
        r.setCaseId(c.getId());
        // 🔴 投影欄一律取自**剛剛鎖住的那一列**，不得由呼叫端指定（複合 FK 會以 23503 擋下謊報）。
        r.setCaseCategory(c.getCategory());
        r.setResolutionType(cmd.resolutionType());
        // 🔴 maker 主張的成因：approve 時會原封不動被拿去結案，覆核者改不了它。
        r.setProposedResponsibilityCause(cmd.proposedResponsibilityCause());
        r.setReason(cmd.reason());
        // 🔴 預覽由 **server 算**，不收呼叫端給的金額（65 §6.4.2 A-4 的 client-provided
        //    impactPreview 已由 PM 裁決撤銷）。算式的兩個輸入都不是呼叫端說了算：
        //      成因      ← 上面剛驗過、即將寫進同一列的 maker 主張
        //      退款基準  ← 交易外從 order-service 讀到的 orders.total_actual_amount
        LocalDateTime computedAt = nowUtc.get();
        ResponsibilityImpactPreview preview = ResponsibilityImpactPreview.of(
                cmd.proposedResponsibilityCause(), fullRefundBasis, computedAt);
        r.setImpactPreview(ResponsibilityImpactPreviewCodec.toJson(objectMapper, preview));
        r.setImpactComputedAt(computedAt);
        // ⚠️ chk_fcr_review_required：requires_review 由 resolutionType 完全決定，不是自由旗標。
        //    責任判定型別 <> ANOMALY_RESUME ⇒ 恆為 true ⇒ 雙人制不得被繞過。
        r.setRequiresReview(Boolean.TRUE);
        r.setStatus(ResponsibilityCaseRules.RESOLUTION_AWAITING_REVIEW);
        r.setProposedByAccountId(cmd.proposedByAccountId());

        if (!resolutions.proposeIfNonePending(r)) {
            throw new IllegalStateException(
                    "responsibility: 該案件已有待覆核提案（caseId=" + c.getId() + "）"
                            + "——同時只能有一筆 AWAITING_REVIEW（uq_fcr_open_per_case）");
        }
        log.info("ResponsibilityCaseService.propose.created",
                "責任判定提案已建立 case_no=" + c.getCaseNo() + " resolution_id=" + r.getId());
        return r.getId();
    }

    // =========================================================================
    // 3. 定案（APPROVE）＋ N16（同一交易）
    // =========================================================================

    /**
     * 定案：{@code 鎖 → CAS resolution → CAS case → INSERT outbox}，<b>四者同一個交易</b>
     * （65 §5.5.2 E0.1、36 §3.3 Atomic Boundary）。
     *
     * <p>🔴 <b>只有兩道 CAS 都 rows=1 才走到 outbox</b>。任一道 {@code rows=0} ⇒ 回
     * {@link Outcome#STALE} 且<b>不發事件</b>——那既是並行下的正確行為，也是 retry 的冪等來源。
     *
     * <p>🔴 結案碼必須是四個 resolved 成因之一。判不出來時的正確處置是讓案件留在
     * {@code IN_PROGRESS} 繼續補證據，<b>不是</b>結案成 {@code RESPONSIBILITY_UNRESOLVED}、
     * 也<b>不是</b> dismiss。
     */
    @Transactional
    public FinalizeResult finalizeResponsibility(FinalizeCommand cmd, long fullRefundBasis) {
        FulfillmentExceptionCaseEntity c = lockResponsibilityCase(cmd.caseId());
        // 🔴 event context 一律取自**鎖住的這一列**（V14），呼叫端無從指定。
        //    守門在建案時就做過了（assertEventContext）＋ chk_exc_responsibility_context
        //    ⇒ 責任案件走到這裡時兩欄必然非空；這裡再驗一次是為了讓「不變式壞了」
        //    當場現形，而不是靜默組出一筆違約 payload。
        ResponsibilityCaseRules.assertEventContext(c.getOrderNo(), c.getShippingMethod());
        if (!ResponsibilityCaseRules.isActionable(c.getStatus())) {
            // 🔴 已 final 的案件再定案一次 ⇒ STALE ＋ 不發事件。這是 L2 冪等的正常路徑，不是錯誤。
            log.info("ResponsibilityCaseService.finalize.alreadyFinal",
                    "案件已非可處理狀態，不重複定案也不發 N16 case_no=" + c.getCaseNo()
                            + " status=" + c.getStatus());
            return new FinalizeResult(Outcome.STALE, c.getCaseNo(), null);
        }

        // 🔴 讀回 <b>maker 的提案</b>並取出他主張的成因（V13）——覆核者提供不了成因。
        //    帶列鎖：讀與 CAS 之間若無鎖，另一個交易可以在這個窗口改掉同一列
        //    ⇒ 結案用的成因與最終落地的那一列不一致。鎖序固定 case → resolution。
        FulfillmentCaseResolutionEntity proposal =
                resolutions.findByIdForUpdate(cmd.resolutionId()).orElse(null);
        if (proposal == null
                || !Objects.equals(proposal.getCaseId(), c.getId())
                || !ResponsibilityCaseRules.isResponsibilityType(proposal.getResolutionType())) {
            // 提案不存在／不屬於這張案件／型別不符 ⇒ 與 ① 的 CAS rows=0 同一個結果。
            // 🔴 STALE 不是成功：呼叫端不得讀成「別人先做過了」就往下走。
            log.info("ResponsibilityCaseService.finalize.proposalMismatch",
                    "提案不存在、不屬於本案件或型別不符，不發 N16 case_no=" + c.getCaseNo()
                            + " resolution_id=" + cmd.resolutionId());
            return new FinalizeResult(Outcome.STALE, c.getCaseNo(), null);
        }

        String cause = proposal.getProposedResponsibilityCause();
        if (!ResponsibilityCaseRules.isFinalizableCause(cause)) {
            // 🔴 責任提案必有合法成因是 DB 保證的（chk_fcr_proposed_cause_binding ＋ _domain）
            //    ⇒ 走到這裡代表有寫入路徑繞過了那兩道 CHECK，屬不變式損壞，必須讓交易 rollback。
            throw new IllegalStateException(
                    "responsibility: 責任提案的 proposed_responsibility_cause 不合法（resolutionId="
                            + cmd.resolutionId() + " cause=" + cause + "）——"
                            + "chk_fcr_proposed_cause_binding／_domain 應使其不可能發生，"
                            + "不得改以覆核者提供的值代替");
        }

        // 🔴 <b>stale guard</b>（65 §6.4.2 硬約束 2）：以**剛剛在交易外讀到的權威退款基準**
        //    重算一次，與提案當下存下的快照比對。不一致 ⇒ 不進任何 CAS、不發 N16。
        //    ⚠️ 只比 refundAmount（業務影響），**不比 computedAt**——重算必然產生新時間戳，
        //       把它納入 equality 會讓每一次覆核都判 stale，那道守衛就永遠是狼來了。
        ResponsibilityImpactPreview stored =
                ResponsibilityImpactPreviewCodec.fromJson(objectMapper, proposal.getImpactPreview());
        ResponsibilityImpactPreview current =
                ResponsibilityImpactPreview.of(cause, fullRefundBasis, nowUtc.get());
        if (!stored.sameBusinessImpactAs(current)) {
            log.info("ResponsibilityCaseService.finalize.stalePreview",
                    "影響預覽已過期，不定案也不發 N16 case_no=" + c.getCaseNo()
                            + " resolution_id=" + cmd.resolutionId()
                            + " stored_refund=" + stored.refundAmount()
                            + " current_refund=" + current.refundAmount());
            return new FinalizeResult(Outcome.STALE_PREVIEW, c.getCaseNo(), null);
        }

        LocalDateTime resolvedAt = nowUtc.get();

        // ① CAS resolution：AWAITING_REVIEW → APPROVED（述詞含 case_id ＋ resolution_type ＋ two-person）
        boolean approved = resolutions.decideResponsibility(
                cmd.resolutionId(), c.getId(),
                ResponsibilityCaseRules.RESOLUTION_AWAITING_REVIEW,
                ResponsibilityCaseRules.RESOLUTION_APPROVED,
                ResponsibilityCaseRules.RESOLUTION_TYPE,
                cmd.reviewedByAccountId(), cmd.reviewComment(), resolvedAt);
        if (!approved) {
            log.info("ResponsibilityCaseService.finalize.stale",
                    "提案已不在 AWAITING_REVIEW／型別不符／覆核者即提案者，不發 N16 case_no="
                            + c.getCaseNo() + " resolution_id=" + cmd.resolutionId());
            return new FinalizeResult(Outcome.STALE, c.getCaseNo(), null);
        }

        // ② CAS case：OPEN／IN_PROGRESS → RESOLVED（結案三件套同一句寫，chk_exc_resolved）
        boolean resolved = cases.resolve(c.getId(), ResponsibilityCaseRules.STATUS_RESOLVED,
                cause, cmd.reviewComment(),
                cmd.reviewedByAccountId(), resolvedAt, ResponsibilityCaseRules.ALLOWED_FROM);
        if (!resolved) {
            // 🔴 我們持有鎖且 ① 剛成功 ⇒ 走到這裡代表不變式壞了。必須讓整個交易 rollback，
            //    否則會留下一個 APPROVED 的提案配一張沒有結案的案件（finality 條件永遠不成立）。
            throw new IllegalStateException(
                    "responsibility: resolution 已 APPROVED 但案件結案 CAS rows=0"
                            + "（caseId=" + c.getId() + " status=" + c.getStatus() + "）"
                            + "——交易必須 rollback，不得留下半套 final 狀態");
        }

        // ③ INSERT outbox（同一交易）
        FulfillmentResponsibilityFinalizedPayload payload =
                new FulfillmentResponsibilityFinalizedPayload(
                        UUID.randomUUID().toString(),
                        MqConstants.EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_FINALIZED,
                        EVENT_VERSION,
                        // 🔴 finality 成立的業務時間，不是發送時間。
                        iso(resolvedAt),
                        currentTraceId(),
                        currentCorrelationId(),
                        c.getOrderNo(),
                        tsidString(c.getShipmentId()),
                        c.getShippingMethod(),
                        c.getSubjectRef(),
                        cause,
                        Boolean.TRUE,
                        c.getCaseNo(),
                        String.valueOf(cmd.resolutionId()),
                        iso(resolvedAt));

        OutboxEventEntity outbox = enqueue(
                MqConstants.EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_FINALIZED,
                MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED,
                payload);
        dispatchAfterCommit(outbox);

        log.info("ResponsibilityCaseService.finalize.applied",
                "責任判定已定案並排入 N16 case_no=" + c.getCaseNo()
                        + " cause=" + cause + " outbox_id=" + outbox.getId());
        return new FinalizeResult(Outcome.APPLIED, c.getCaseNo(), outbox.getId());
    }

    // =========================================================================
    // 4. 駁回（案件不動）
    // =========================================================================

    /**
     * 駁回提案。
     *
     * <p>🔴 <b>案件維持 {@code IN_PROGRESS}、owner 保留、不 DISMISSED、不 RESOLVED、不另開第二張</b>
     * （65 §5.5.2 E0.2 transition 表逐字）。正確的續行方式是<b>在同一張 case 上重提</b>——
     * {@code uq_fcr_open_per_case} 只擋 {@code AWAITING_REVIEW}，前一筆已 REJECTED ⇒ 新提案寫得進去。
     *
     * <p>🔴 <b>不發任何事件</b>：駁回不是 financially final，N16 的唯一產生點是 APPROVED ＋ RESOLVED。
     */
    @Transactional
    public Outcome reject(RejectCommand cmd) {
        FulfillmentExceptionCaseEntity c = lockResponsibilityCase(cmd.caseId());
        if (!ResponsibilityCaseRules.isActionable(c.getStatus())) {
            // 與定案同理：重送一次駁回請求是 at-least-once 世界的常態，不得變成 500。
            return Outcome.STALE;
        }

        boolean rejected = resolutions.decideResponsibility(
                cmd.resolutionId(), c.getId(),
                ResponsibilityCaseRules.RESOLUTION_AWAITING_REVIEW,
                ResponsibilityCaseRules.RESOLUTION_REJECTED,
                ResponsibilityCaseRules.RESOLUTION_TYPE,
                cmd.reviewedByAccountId(), cmd.reviewComment(), nowUtc.get());
        if (!rejected) {
            return Outcome.STALE;
        }
        log.info("ResponsibilityCaseService.reject.applied",
                "責任判定提案已駁回，案件維持可再提案 case_no=" + c.getCaseNo()
                        + " status=" + c.getStatus() + " resolution_id=" + cmd.resolutionId());
        return Outcome.APPLIED;
    }

    // =========================================================================
    // 內部
    // =========================================================================

    /**
     * 鎖住案件並確認它是一張責任判定案件。
     *
     * <p>🔴 fail-closed：查不到、或不是責任判定 category，一律拒絕（那是<b>錯誤</b>，不是狀態問題）。
     *
     * <p>⚠️ <b>本方法刻意不驗 allowed-from</b>：「案件已不在可處理狀態」對三個 caller 的意義不同——
     * <ul>
     *   <li><b>提案</b>：那是操作錯誤（對一張已定案的案件提新方案）⇒ 拋例外</li>
     *   <li><b>定案／駁回</b>：那是 <b>retry 的正常結果</b>（36 §3.3 的 L2 逐字：「commit 之後的 retry
     *       讀到已是 final ⇒ CAS {@code rows=0} ⇒ 不會 INSERT 第二筆」）⇒ 回 {@link Outcome#STALE}</li>
     * </ul>
     * 把它一律做成例外，會讓「重送一次定案請求」變成 500，而那是<b>預期會發生</b>的事
     * （at-least-once 的世界裡，重試是常態不是異常）。
     */
    private FulfillmentExceptionCaseEntity lockResponsibilityCase(long caseId) {
        Optional<FulfillmentExceptionCaseEntity> found = cases.findByIdForUpdate(caseId);
        FulfillmentExceptionCaseEntity c = found.orElseThrow(() -> new IllegalStateException(
                "responsibility: 案件不存在（caseId=" + caseId + "）"));
        if (!ResponsibilityCaseRules.isResponsibilityCategory(c.getCategory())) {
            throw new IllegalStateException(
                    "responsibility: 這不是責任判定案件（caseId=" + caseId
                            + " category=" + c.getCategory() + "）——本 service 的每一條規則都只適用於 "
                            + ResponsibilityCaseRules.CATEGORY);
        }
        return c;
    }

    /** 序列化並在<b>當前交易</b>內寫入 outbox。序列化失敗一律 rethrow 讓交易 rollback。 */
    private OutboxEventEntity enqueue(String eventType, String routingKey, Object payload) {
        String json;
        try {
            json = objectMapper.writeValueAsString(payload);
        } catch (JsonProcessingException ex) {
            // 🔴 禁止 catch + return：必須讓交易 rollback，避免狀態落地而事件漏寫。
            throw new IllegalStateException(
                    "responsibility: 事件 payload 序列化失敗 " + eventType + " - " + ex.getMessage(), ex);
        }
        // 🔴 exchange 顯式傳入 hawkr.shipping：generic path 自己不知道要發去哪，
        //    而 payment.events 上沒有這兩條 routing key 的 binding ⇒ 發錯會 unroutable 且無聲。
        return outboxWriter.enqueue(MqConstants.EXCHANGE_SHIPPING, eventType, routingKey, json);
    }

    /**
     * commit 成功後於原執行緒即時投遞（36 §3.3：afterCommit 即時履約、排程器每 5s 兜底）。
     *
     * <p>🔴 例外全吞不 rethrow：afterCommit 上拋會傳到 commit 呼叫方，造成「判定已定案卻對
     * 呼叫端回錯」。投遞失敗只留 PENDING，交 {@code OutboxEventPublisherScheduler} 兜底。
     */
    private void dispatchAfterCommit(OutboxEventEntity entity) {
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                try {
                    outboxPublisher.publish(entity);
                } catch (Throwable t) {
                    log.error("ResponsibilityCaseService.afterCommit.dispatchFailed",
                            "afterCommit 即時投遞失敗，交排程器兜底 outbox_id=" + entity.getId()
                                    + " routing_key=" + entity.getRoutingKey(), t);
                }
            }
        });
    }

    /**
     * TSID → wire 字串；{@code null} 保持 {@code null}。
     *
     * <p>🔴 <b>這是 internal 型別與 wire 契約的唯一轉換點</b>（65 §6A.1.3：新事件的 id 一律
     * {@code string}）。domain／command 側維持 {@code Long} 與 DB 一致，只在這裡跨邊界。
     * <p>⚠️ 不得改用 {@code String.valueOf(Long)}——它會把 {@code null} 變成字面字串 {@code "null"}，
     * 而那在 wire 上是一個<b>看起來像值的東西</b>，consumer 分不出「沒有 shipment」與「有一個叫 null 的」。
     */
    private static String tsidString(Long id) {
        return id == null ? null : Long.toString(id);
    }

    private static String iso(LocalDateTime utc) {
        return utc == null ? null : utc.atOffset(ZoneOffset.UTC).format(ISO_UTC);
    }

    /** trace/correlation 為觀測性透傳欄，取不到時回空字串（36：空不是錯誤）。 */
    private static String currentTraceId() {
        String v = org.slf4j.MDC.get("trace_id");
        return v == null ? "" : v;
    }

    private static String currentCorrelationId() {
        String v = org.slf4j.MDC.get("correlation_id");
        return v == null ? "" : v;
    }
}
