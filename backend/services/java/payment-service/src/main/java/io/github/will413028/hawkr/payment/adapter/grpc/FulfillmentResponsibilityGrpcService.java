package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.grpc.GetOrderForPaymentResponse;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.grpc.FulfillmentResponsibilityServiceGrpc;
import io.github.will413028.hawkr.payment.grpc.ProposeResponsibilityResolutionForAdminData;
import io.github.will413028.hawkr.payment.grpc.ProposeResponsibilityResolutionForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ProposeResponsibilityResolutionForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ReviewResponsibilityResolutionForAdminData;
import io.github.will413028.hawkr.payment.grpc.ReviewResponsibilityResolutionForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ReviewResponsibilityResolutionForAdminResponse;
import io.github.will413028.hawkr.payment.logistics.application.ResponsibilityCaseService;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityImpactPreview;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityImpactPreviewCodec;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentCaseResolutionRepository;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.stub.StreamObserver;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import net.devh.boot.grpc.server.service.GrpcService;

/**
 * 責任判定 maker-checker 的 gRPC adapter（WP2B）。
 *
 * <p><b>本類別做什麼</b>：把 ADM 的 A-4／A-5 兩個動作接到既有的
 * {@link ResponsibilityCaseService} primitive 上，並負責三件 adapter 層的事——
 * <b>授權</b>、<b>交易外取權威退款基準</b>、<b>把 domain 結果映成業務碼</b>。
 *
 * <p>🔴 <b>本類別不含任何業務規則。</b> 成因值域、allowed-from、序列化、stale guard、
 * 事件排出的時機，全部住在 {@link ResponsibilityCaseService} 與
 * {@link ResponsibilityCaseRules}（而它們每一條在 DB 都有對應的 CHECK／UNIQUE 兜底）。
 * 在這一層再寫一份判斷，就會多出一份會漂移的真相。
 *
 * <h2>兩段式：為什麼退款基準必須在交易外取</h2>
 *
 * {@code propose} 與 {@code finalizeResponsibility} 都把 {@code fullRefundBasis} 當成
 * <b>位置參數</b>傳入，而不是自己去查。這不是風格：
 *
 * <ul>
 *   <li>那個值的權威在 <b>order-service</b>（{@code orders.total_actual_amount}），
 *       取它必然是一次跨服務 gRPC</li>
 *   <li>36 §3.3.1 明令<b>交易內不得做 gRPC enrich</b>——責任判定的交易裡還有
 *       {@code SELECT … FOR UPDATE} 持有的 case 列鎖，把一次可能逾時 10 秒的網路呼叫
 *       放進去，等於讓外部服務的延遲決定我方鎖的持有時間</li>
 * </ul>
 *
 * ⇒ 順序固定：<b>① 無鎖讀 case 取 order_no ② gRPC 取退款基準 ③ 才進交易</b>。
 *
 * <p>⚠️ <b>① 拿到的是快照，而這是安全的</b>：{@code case_no} 由 {@code uq_exc_no} 保證唯一
 * 且永不改變，{@code order_no} 由 V14 凍結在建案當下、全 repo 無更新路徑
 * ⇒ 交易內 {@code findByIdForUpdate} 重讀到的必然是同一列、同一個 {@code order_no}。
 * 會變的只有 {@code status}，而那正是交易內 CAS 述詞在判的東西。
 *
 * <h2>🔴 三件不得由呼叫端提供的東西</h2>
 *
 * <table border="1">
 *   <caption>它們的來源與「由 client 給」的後果</caption>
 *   <tr><th>值</th><th>真正的來源</th><th>由 client 給會怎樣</th></tr>
 *   <tr><td>操作者 account_id</td><td>{@link GrpcAuthContext#getAccountId()}</td>
 *       <td>呼叫端可宣稱自己是誰 ⇒ {@code chk_fcr_two_person} 形同虛設，
 *           一個人用兩個 id 就能自問自答</td></tr>
 *   <tr><td>影響預覽金額</td><td>server 依成因 ＋ {@code total_actual_amount} 算</td>
 *       <td>被覆核的數字由被覆核者自己決定</td></tr>
 *   <tr><td>責任成因（覆核時）</td><td>maker 的提案列（V13）</td>
 *       <td>覆核者可以批准一個沒有人提過的成因 ⇒ 雙人制只剩兩個簽名</td></tr>
 * </table>
 *
 * <h2>本波只支援責任判定型別</h2>
 *
 * {@code chk_fcr_type} 另外六個型別（{@code CLAIM_SETTLE}／{@code CLAIM_DISMISS}／
 * {@code SHIPMENT_CANCEL}／{@code SHIPMENT_CANCEL_CONFIRM}／
 * {@code SHIPMENT_CANCEL_AND_REFUND}／{@code ANOMALY_RESUME}）的副作用全部落在
 * shipping-service 的配送軸狀態邊上（65 對照表），本波不實作。
 * ⇒ 一律 <b>fail-closed 拒絕（9571）</b>，而不是先收下來——收下一個沒有人會執行副作用的
 * 提案，等於製造一張永遠結不了的案件，而失效形狀是完全靜默的。
 */
@GrpcService
public class FulfillmentResponsibilityGrpcService
        extends FulfillmentResponsibilityServiceGrpc.FulfillmentResponsibilityServiceImplBase {

    private static final ObsLogger log =
            ObsLogger.forClass(FulfillmentResponsibilityGrpcService.class);

    private static final DateTimeFormatter ISO_UTC =
            DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'");

    private static final String DECISION_APPROVE = "APPROVE";
    private static final String DECISION_REJECT = "REJECT";

    private final ResponsibilityCaseService responsibilityCases;
    private final FulfillmentExceptionCaseRepository cases;
    private final FulfillmentCaseResolutionRepository resolutions;
    private final OrderGrpcClient orderGrpcClient;
    /** 只用於回讀已持久化的 {@code impact_preview}（不參與任何寫入路徑）。 */
    private final ObjectMapper objectMapper;

    public FulfillmentResponsibilityGrpcService(ResponsibilityCaseService responsibilityCases,
                                                FulfillmentExceptionCaseRepository cases,
                                                FulfillmentCaseResolutionRepository resolutions,
                                                OrderGrpcClient orderGrpcClient,
                                                ObjectMapper objectMapper) {
        this.responsibilityCases = responsibilityCases;
        this.cases = cases;
        this.resolutions = resolutions;
        this.orderGrpcClient = orderGrpcClient;
        this.objectMapper = objectMapper;
    }

    // =========================================================================
    // A-4 maker：提案
    // =========================================================================

    @Override
    public void proposeResponsibilityResolutionForAdmin(
            ProposeResponsibilityResolutionForAdminRequest request,
            StreamObserver<ProposeResponsibilityResolutionForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long actor = GrpcAuthContext.getAccountId();

            FulfillmentExceptionCaseEntity c = cases.findByCaseNo(nz(request.getCaseNo())).orElse(null);
            if (c == null) {
                observer.onNext(proposeErr(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND));
                observer.onCompleted();
                return;
            }
            // 🔴 本波只走責任判定軸。非責任案件的處置型別副作用不在本波射程內
            //    ⇒ fail-closed，不得先收下提案。
            if (!ResponsibilityCaseRules.isResponsibilityCategory(c.getCategory())
                    || !ResponsibilityCaseRules.isResponsibilityType(nz(request.getResolutionType()))) {
                log.info("FulfillmentResponsibilityGrpcService.propose.unsupportedType",
                        "本波僅支援責任判定提案 case_no=" + c.getCaseNo()
                                + " category=" + c.getCategory()
                                + " resolution_type=" + request.getResolutionType());
                observer.onNext(proposeErr(GrpcResponseCode.EXCEPTION_CASE_STATE_INVALID));
                observer.onCompleted();
                return;
            }
            // 🔴 稽核必要：一個沒有理由的責任判定，事後沒有人答得出「為什麼這樣判」。
            if (nz(request.getReason()).isBlank()) {
                observer.onNext(proposeErr(GrpcResponseCode.INVALID_PARAM));
                observer.onCompleted();
                return;
            }
            // 成因值域交給 domain 守（ResponsibilityCaseRules.assertProposableCause），
            // 這一層不再寫第二份判斷。

            long basis = fullRefundBasis(c);

            long resolutionId = responsibilityCases.propose(
                    new ResponsibilityCaseService.ProposeCommand(
                            c.getId(),
                            request.getResolutionType(),
                            request.getResponsibilityCause(),
                            request.getReason(),
                            actor),
                    basis);

            // 🔴 **回讀被持久化的那一列**，不在這裡重算一份給 ADM 看。
            //    重算出來的 computedAt 與 DB 內的必然差幾毫秒，而 ADM 之後按「覆核」時，
            //    stale guard 比的是 **DB 裡那一份**。兩個數字只要有任何一天不一致，
            //    畫面上的預覽就不是被保護的那一份——而 maker-checker 保護的正是
            //    「覆核者看到的金額 ＝ 實際會執行的金額」這件事本身（65 硬約束 2）。
            var stored = cases.findByCaseNo(c.getCaseNo()).orElse(c);
            var persisted = resolutions.findById(resolutionId).orElse(null);
            ResponsibilityImpactPreview preview = persisted == null ? null
                    : ResponsibilityImpactPreviewCodec.fromJson(objectMapper, persisted.getImpactPreview());
            ProposeResponsibilityResolutionForAdminData.Builder data =
                    ProposeResponsibilityResolutionForAdminData.newBuilder()
                            .setResolutionId(String.valueOf(resolutionId))
                            .setCaseNo(nz(stored.getCaseNo()))
                            .setCaseStatus(nz(stored.getStatus()))
                            .setImpactPreviewRefundAmount(
                                    preview == null ? "" : String.valueOf(preview.refundAmount()))
                            .setImpactComputedAt(
                                    persisted == null ? "" : iso(persisted.getImpactComputedAt()));
            observer.onNext(ProposeResponsibilityResolutionForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (IllegalArgumentException e) {
            // domain 的值域／fail-closed 拒絕（成因不合法、型別與 category 不相容…）。
            // 🔴 這是**操作錯誤**不是系統錯誤，不得讓它走成 500。
            log.info("FulfillmentResponsibilityGrpcService.propose.rejected",
                    "提案被 domain 拒絕：" + e.getMessage());
            observer.onNext(proposeErr(GrpcResponseCode.INVALID_PARAM));
            observer.onCompleted();
        } catch (IllegalStateException e) {
            // 案件已不可提案（RESOLVED）／已有待覆核提案（uq_fcr_open_per_case）。
            log.info("FulfillmentResponsibilityGrpcService.propose.stale",
                    "提案被狀態守衛擋下：" + e.getMessage());
            observer.onNext(proposeErr(GrpcResponseCode.EXCEPTION_CASE_STATE_INVALID));
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ProposeResponsibilityResolutionForAdminResponse.newBuilder(), e,
                    "FulfillmentResponsibilityGrpcService", "proposeResponsibilityResolutionForAdmin");
        }
    }

    // =========================================================================
    // A-5 checker：覆核
    // =========================================================================

    @Override
    public void reviewResponsibilityResolutionForAdmin(
            ReviewResponsibilityResolutionForAdminRequest request,
            StreamObserver<ReviewResponsibilityResolutionForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long actor = GrpcAuthContext.getAccountId();

            String decision = nz(request.getDecision()).trim().toUpperCase();
            if (!DECISION_APPROVE.equals(decision) && !DECISION_REJECT.equals(decision)) {
                // 🔴 未知 decision 不得預設成任何一邊：預設 APPROVE 會讓打錯字變成核准不退款，
                //    預設 REJECT 會讓核准靜默失效。
                observer.onNext(reviewErr(GrpcResponseCode.INVALID_PARAM));
                observer.onCompleted();
                return;
            }
            long resolutionId = parseResolutionId(request.getResolutionId());
            if (resolutionId <= 0) {
                observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_RESOLUTION_NOT_FOUND));
                observer.onCompleted();
                return;
            }
            FulfillmentExceptionCaseEntity c = cases.findByCaseNo(nz(request.getCaseNo())).orElse(null);
            if (c == null) {
                observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND));
                observer.onCompleted();
                return;
            }
            if (!ResponsibilityCaseRules.isResponsibilityCategory(c.getCategory())) {
                observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_STATE_INVALID));
                observer.onCompleted();
                return;
            }

            if (DECISION_REJECT.equals(decision)) {
                // 🔴 駁回不動任何業務狀態、不發任何事件、不 DISMISS 案件。
                //    案件維持 IN_PROGRESS、owner 保留，原 maker 可修正後在同一張 case 重提。
                ResponsibilityCaseService.Outcome outcome = responsibilityCases.reject(
                        new ResponsibilityCaseService.RejectCommand(
                                c.getId(), resolutionId, request.getReviewComment(), actor));
                if (outcome == ResponsibilityCaseService.Outcome.STALE) {
                    observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_RESOLUTION_ALREADY_REVIEWED));
                    observer.onCompleted();
                    return;
                }
                var after = cases.findByCaseNo(c.getCaseNo()).orElse(c);
                observer.onNext(reviewOk(ReviewResponsibilityResolutionForAdminData.newBuilder()
                        .setCaseNo(nz(after.getCaseNo()))
                        .setResolutionId(String.valueOf(resolutionId))
                        .setResolutionStatus(ResponsibilityCaseRules.RESOLUTION_REJECTED)
                        .setCaseStatus(nz(after.getStatus()))
                        // 🔴 REJECT 不產生成因：沒有任何東西被定案。
                        .setResponsibilityCause("")
                        .setN16Enqueued(false)));
                observer.onCompleted();
                return;
            }

            // ── APPROVE ──────────────────────────────────────────────────────
            long basis = fullRefundBasis(c);
            ResponsibilityCaseService.FinalizeResult result = responsibilityCases.finalizeResponsibility(
                    new ResponsibilityCaseService.FinalizeCommand(
                            c.getId(), resolutionId, request.getReviewComment(), actor),
                    basis);

            if (result.outcome() == ResponsibilityCaseService.Outcome.STALE_PREVIEW) {
                // 🔴 這一格與 STALE 不是同一件事：狀態沒問題，但金額前提變了。
                //    合成同一個碼會讓 ADM 分不出「別人先做過了」與「請重新產生預覽」。
                log.info("FulfillmentResponsibilityGrpcService.review.stalePreview",
                        "影響預覽已過期，未定案也未發 N16 case_no=" + c.getCaseNo());
                observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_IMPACT_PREVIEW_STALE));
                observer.onCompleted();
                return;
            }

            var after = cases.findByCaseNo(c.getCaseNo()).orElse(c);
            if (result.outcome() == ResponsibilityCaseService.Outcome.STALE) {
                // 🔴 STALE 有兩種來源，對 ADM 的意義不同，所以用當下的案件狀態分辨：
                //    ① 案件已 RESOLVED ⇒ 這是**重送**，冪等成功（不發第二筆 N16）
                //    ② 案件仍可處理 ⇒ 是**這一筆提案**已被覆核過／不存在／不屬於本案件
                if (ResponsibilityCaseRules.STATUS_RESOLVED.equals(after.getStatus())) {
                    log.info("FulfillmentResponsibilityGrpcService.review.replay",
                            "案件已定案，重送不發第二筆 N16 case_no=" + after.getCaseNo());
                    observer.onNext(reviewOk(ReviewResponsibilityResolutionForAdminData.newBuilder()
                            .setCaseNo(nz(after.getCaseNo()))
                            .setResolutionId(String.valueOf(resolutionId))
                            .setResolutionStatus(ResponsibilityCaseRules.RESOLUTION_APPROVED)
                            .setCaseStatus(nz(after.getStatus()))
                            .setResponsibilityCause(nz(after.getResolutionCode()))
                            .setN16Enqueued(false)));
                    observer.onCompleted();
                    return;
                }
                observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_RESOLUTION_ALREADY_REVIEWED));
                observer.onCompleted();
                return;
            }

            log.info("FulfillmentResponsibilityGrpcService.review.applied",
                    "責任判定已定案並排入 N16 case_no=" + after.getCaseNo()
                            + " cause=" + after.getResolutionCode()
                            + " outbox_id=" + result.outboxId());
            observer.onNext(reviewOk(ReviewResponsibilityResolutionForAdminData.newBuilder()
                    .setCaseNo(nz(after.getCaseNo()))
                    .setResolutionId(String.valueOf(resolutionId))
                    .setResolutionStatus(ResponsibilityCaseRules.RESOLUTION_APPROVED)
                    .setCaseStatus(nz(after.getStatus()))
                    .setResponsibilityCause(nz(after.getResolutionCode()))
                    .setN16Enqueued(true)));
            observer.onCompleted();
        } catch (IllegalArgumentException e) {
            log.info("FulfillmentResponsibilityGrpcService.review.rejected",
                    "覆核被 domain 拒絕：" + e.getMessage());
            observer.onNext(reviewErr(GrpcResponseCode.INVALID_PARAM));
            observer.onCompleted();
        } catch (IllegalStateException e) {
            log.info("FulfillmentResponsibilityGrpcService.review.invalidState",
                    "覆核被狀態守衛擋下：" + e.getMessage());
            observer.onNext(reviewErr(GrpcResponseCode.EXCEPTION_CASE_STATE_INVALID));
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ReviewResponsibilityResolutionForAdminResponse.newBuilder(), e,
                    "FulfillmentResponsibilityGrpcService", "reviewResponsibilityResolutionForAdmin");
        }
    }

    // =========================================================================
    // 內部
    // =========================================================================

    /**
     * 取權威全額退款基準 ＝ {@code orders.total_actual_amount}（<b>交易外</b>）。
     *
     * <p>🔴 <b>fail-closed</b>：讀不到就拋，<b>不得</b>退回 0。退回 0 會讓每一筆判定的
     * 預覽都變成「退 0 元」——那與 {@code BUYER_CAUSED} 的正確結果<b>長得一模一樣</b>，
     * 而它對其餘三個成因是完全錯的（該全額退卻顯示不退）。覆核者會核准一個他看不見的金額。
     *
     * <p>⚠️ 這也是為什麼它<b>不能</b> best-effort：這個值同時決定提案時寫進 DB 的快照與
     * 覆核時 stale guard 的比對基準。兩次取到不同的「安全預設值」，守衛就會在錯的時候響、
     * 也會在該響的時候不響。
     */
    private long fullRefundBasis(FulfillmentExceptionCaseEntity c) {
        String orderNo = c.getOrderNo();
        if (orderNo == null || orderNo.isBlank()) {
            // V14 的 chk_exc_responsibility_context 保證責任案件必有 order_no
            // ⇒ 走到這裡代表不變式壞了，必須現形而不是繼續算。
            throw new IllegalStateException(
                    "responsibility: 責任判定案件缺 order_no（case_no=" + c.getCaseNo() + "）"
                            + "——chk_exc_responsibility_context 應使其不可能發生");
        }
        GetOrderForPaymentResponse resp = orderGrpcClient.getOrderForPayment(orderNo);
        if (resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new IllegalStateException(
                    "responsibility: 取不到訂單退款基準 order_no=" + orderNo
                            + " code=" + resp.getCode() + " msg=" + resp.getMessage()
                            + "——不得以 0 代替，那會讓非 BUYER_CAUSED 的預覽顯示成不退款");
        }
        String amount = resp.getData().getAmount();
        try {
            long basis = Long.parseLong(amount.trim());
            if (basis < 0) {
                throw new NumberFormatException("negative");
            }
            return basis;
        } catch (RuntimeException e) {
            throw new IllegalStateException(
                    "responsibility: 訂單退款基準不是合法金額 order_no=" + orderNo
                            + " amount='" + amount + "'", e);
        }
    }

    private static long parseResolutionId(String raw) {
        try {
            return Long.parseLong(nz(raw).trim());
        } catch (RuntimeException e) {
            return 0L;
        }
    }

    private static String nz(String s) {
        return s == null ? "" : s;
    }

    private static String iso(LocalDateTime t) {
        return t == null ? "" : t.format(ISO_UTC);
    }

    private static ProposeResponsibilityResolutionForAdminResponse proposeErr(GrpcResponseCode code) {
        return ProposeResponsibilityResolutionForAdminResponse.newBuilder()
                .setCode(code.code()).setMessage(code.message()).build();
    }

    private static ReviewResponsibilityResolutionForAdminResponse reviewErr(GrpcResponseCode code) {
        return ReviewResponsibilityResolutionForAdminResponse.newBuilder()
                .setCode(code.code()).setMessage(code.message()).build();
    }

    private static ReviewResponsibilityResolutionForAdminResponse reviewOk(
            ReviewResponsibilityResolutionForAdminData.Builder data) {
        return ReviewResponsibilityResolutionForAdminResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage(GrpcResponseCode.SUCCESS.message())
                .setData(data)
                .build();
    }
}
