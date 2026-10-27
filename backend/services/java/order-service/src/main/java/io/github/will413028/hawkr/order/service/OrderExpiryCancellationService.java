package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.PaymentEventHandler;
import io.github.will413028.hawkr.order.util.HashUtil;
import io.github.will413028.hawkr.payment.grpc.GetActivePaymentStatusByOrderNoResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.stereotype.Service;

/**
 * PENDING_PAYMENT 訂單系統批次取消的共用編排（03 §5.1a.2 / §8.5）。
 *
 * <p>抽出自 {@code PaymentDeadlineScheduler}（付款逾時掃描的原始 caller），供賣家停權批次取消
 * （{@code SellerSuspendedConsumer}）複用同一套「取消前反查付款 SoT 收斂 + TX 內悲觀鎖 CAS +
 * TX 外 best-effort 補償」三段式防護，零新增機制（46 §5）。
 *
 * <p>不可與 {@link PaymentEventHandler} 合併：{@link #convergeIfAlreadyPaid} 需透過 Spring proxy
 * 呼叫 {@code paymentEventHandler.handlePaymentSuccess}（@Transactional），同 class 內
 * self-invocation 會繞過 proxy 導致交易邊界失效（同 {@code SellerProductOffShelfService} 註解的
 * CGLIB 限制）。
 */
@Service
public class OrderExpiryCancellationService {

    private static final ObsLogger log = ObsLogger.forClass(OrderExpiryCancellationService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final PaymentGrpcClient paymentGrpcClient;
    private final PaymentEventHandler paymentEventHandler;
    private final CheckoutCouponGrpcClient checkoutCouponGrpcClient;

    public OrderExpiryCancellationService(
            PaymentGrpcClient paymentGrpcClient,
            PaymentEventHandler paymentEventHandler,
            CheckoutCouponGrpcClient checkoutCouponGrpcClient
    ) {
        this.paymentGrpcClient = paymentGrpcClient;
        this.paymentEventHandler = paymentEventHandler;
        this.checkoutCouponGrpcClient = checkoutCouponGrpcClient;
    }

    /**
     * 取消單筆 PENDING_PAYMENT 訂單（付款逾時排程 / 賣家停權批次取消 共用入口）。
     *
     * @param orderNo      待取消訂單號
     * @param cancelReason {@code OrderCancelReason.value()}（03 §5.3.1；由呼叫端決定語意，本方法
     *                     只透傳，寫入前的合法性驗證在 {@link PaymentEventHandler#cancelExpiredOrderInTx}）
     * @return true 表示本次呼叫實際取消了訂單；false 表示已被其他流程處理，或付款已成功並收斂為 PAID
     */
    public boolean cancelPendingOrder(String orderNo, String cancelReason) {
        // 階段 0（TX 外，#1349 L3 縱深防護，03 §8.5）：取消前反查 payment SoT，已付款收斂為 PAID、
        // 付款真相未知（RPC 失敗/非成功碼）則保守跳過，皆不取消（fail-safe，MQ-01 窄窗修復）。
        if (convergeIfAlreadyPaid(orderNo)) {
            return false;
        }

        // 階段 1：TX 內悲觀鎖 + CAS 狀態變更
        PaymentEventHandler.CancelInfo info = paymentEventHandler.cancelExpiredOrderInTx(orderNo, cancelReason);
        if (info == null) {
            return false; // 已被其他流程處理（狀態已變更）
        }

        // 階段 2：TX 外 best-effort 補償
        compensateAfterCancel(info);
        return true;
    }

    /**
     * 取消前反查付款 SoT（03 §8.5，cancel 動作的同步前置一致性 gate，非 reconcile job）。
     *
     * <p>回傳 {@code true} = 本輪跳過取消，三種情境：
     * <ul>
     *   <li>PAID → 以 payment {@link PaymentDetail} 組 payload 走既有冪等
     *       {@link PaymentEventHandler#handlePaymentSuccess} 收斂 order → PAID。
     *   <li>RPC 失敗（gate 連不上 payment）→ 保守跳過，待下輪排程 gate 成功再判。
     *   <li>回非成功碼（payment 側查詢自身失敗）→ 同上保守跳過。
     * </ul>
     * <p>回傳 {@code false} = 照舊取消，僅限 gate <b>確定判定</b>付款非 PAID（含查無交易＝成功回空 data）。
     *
     * <p>設計取捨（fail-safe，MQ-01 窄窗修復）：付款真相「未知」時一律不取消。寧可讓逾期單延後到
     * 下輪重掃（payment 持續故障期間庫存續佔），也不因 gate 查詢失敗而誤取消實際已付款訂單
     * （誤取消已付款單＝資損＋客訴）。跳過的訂單仍 PENDING_PAYMENT，由 PaymentDeadlineScheduler
     * 每輪重掃自癒，無需補償 job（§22）。
     */
    private boolean convergeIfAlreadyPaid(String orderNo) {
        GetActivePaymentStatusByOrderNoResponse response;
        try {
            response = paymentGrpcClient.getActivePaymentStatusByOrderNo(orderNo);
        } catch (Exception e) {
            // fail-safe（MQ-01）：gate 無法判定付款真相，保守跳過取消，待下輪排程 gate 成功再判
            log.warn("OrderExpiryCancellationService.gate.rpcFailed",
                    "取消前反查付款狀態失敗，保守跳過取消待下輪重試: orderNo=" + orderNo + ", err=" + e.getMessage());
            return true;
        }

        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            // fail-safe（MQ-01）：payment 側查詢自身失敗，付款真相未知，同 RPC 失敗保守跳過
            log.warn("OrderExpiryCancellationService.gate.nonSuccess",
                    "取消前反查回非成功碼，保守跳過取消待下輪重試: orderNo=" + orderNo + ", code=" + response.getCode());
            return true;
        }

        PaymentDetail detail = response.getData();
        if (!PaymentTransactionStatus.PAID.value().equals(detail.getStatus())) {
            return false; // 非 PAID / 查無交易 → 照舊取消
        }

        JsonNode payload = buildPaymentSuccessPayload(orderNo, detail);
        String body = payload.toString();
        paymentEventHandler.handlePaymentSuccess(payload, body, HashUtil.sha256Hex(body));
        log.info("OrderExpiryCancellationService.gate.converged",
                "訂單付款已成功，跳過取消並收斂為 PAID: tradeNo=" + detail.getTradeNo(),
                "orderNo", orderNo);
        return true;
    }

    /**
     * 以 payment {@link PaymentDetail} 組出與 payment.success MQ event 同形狀的 payload，
     * 沿用 {@link PaymentEventHandler#handlePaymentSuccess} 既讀的同名欄位。
     */
    private JsonNode buildPaymentSuccessPayload(String orderNo, PaymentDetail detail) {
        ObjectNode payload = MAPPER.createObjectNode();
        payload.put("trade_no", detail.getTradeNo());
        payload.put("order_id", detail.getOrderId().isEmpty() ? orderNo : detail.getOrderId());
        payload.put("amount", parseAmount(detail.getAmount()));
        payload.put("pay_time", toIsoPayTime(detail.getPayTime()));
        payload.put("payment_type", detail.getPaymentType());
        return payload;
    }

    private static long parseAmount(String amountStr) {
        try {
            return (amountStr == null || amountStr.isEmpty()) ? 0L : Long.parseLong(amountStr.trim());
        } catch (NumberFormatException e) {
            return 0L;
        }
    }

    /** PaymentDetail.pay_time 為 Unix epoch 毫秒字串；轉為 handlePaymentSuccess 期望的 ISO LocalDateTime 字串。 */
    private static String toIsoPayTime(String epochMillisStr) {
        try {
            if (epochMillisStr == null || epochMillisStr.isEmpty()) {
                return ""; // handlePaymentSuccess.parsePayTime 會 fallback 為 nowUtc()
            }
            return DateTimeUtils.fromEpochMillis(Long.parseLong(epochMillisStr.trim())).toString();
        } catch (NumberFormatException e) {
            return "";
        }
    }

    /**
     * TX 外 best-effort 補償：優惠券走 gRPC（庫存歸還已於 cancelExpiredOrderInTx 同事務落 order.cancelled outbox，#1368）
     */
    private void compensateAfterCancel(PaymentEventHandler.CancelInfo info) {
        // 庫存歸還已改為取消事務內落 order.cancelled outbox（#1368），由 outbox scheduler 保證投遞，此處不再直發

        // 禁止呼叫 ClosePaymentByOrderNo：讓交易由 Payment Service ExpiredTransactionScheduler 自然過期，
        // 確保遲到的 PayUni Webhook 能正常觸發退款補償流程（見設計書 §8.5）

        // 釋放優惠券（失敗由 Coupon AutoReleaseJob 兜底）
        try {
            String couponInstanceId = info.couponInstanceId();
            if (couponInstanceId != null && !couponInstanceId.isEmpty()) {
                checkoutCouponGrpcClient.releaseCoupon(couponInstanceId, info.orderNo());
            }
        } catch (Exception e) {
            log.error("OrderExpiryCancellationService.releaseCoupon.error", "釋放優惠券失敗", e);
        }
    }
}
