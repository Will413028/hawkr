package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.AppType;
import io.github.will413028.hawkr.common.utils.code.ClientType;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.helper.PaymentMethodLimitRules;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.order.grpc.GetOrderForPaymentResponse;
import io.github.will413028.hawkr.order.grpc.OrderPaymentInfo;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.application.dto.CreatePaymentRequest;
import io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.application.dto.PaymentDetailResponse;
import io.github.will413028.hawkr.payment.domain.GrantResponseRules;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payuni.http.PayUniApiException;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payuni.payment.model.UppFormData;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentLogEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Isolation;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Optional;

/**
 * 支付應用服務
 *
 * 協調領域服務、Repository、外部客戶端完成業務用例
 */
@Service
public class PaymentApplicationService {

    private static final ObsLogger log = ObsLogger.forClass(PaymentApplicationService.class);
    private static final String TRIGGER_SOURCE_GRANT_API = "GRANT_API";

    // ===== §5.6 去重硬化 B：grant_query 對帳參數 =====
    /** PayUni 撥款結算時區（台北）；grant_query QueryDate 以此換算日曆日。 */
    private static final ZoneId PAYUNI_ZONE = ZoneId.of("Asia/Taipei");
    /** grant_query 僅可查 30 天內（代理商專用 API §6.3）；對帳區間上限。 */
    private static final int GRANT_QUERY_MAX_LOOKBACK_DAYS = 30;
    /** grant_query 每頁 500 筆；單日分頁防呆上限，避免 PayUni 回異常 Total 造成過量呼叫。 */
    private static final int GRANT_QUERY_MAX_PAGES_PER_DAY = 200;
    private static final DateTimeFormatter GRANT_QUERY_DATE_FMT = DateTimeFormatter.ofPattern("yyyy-MM-dd");

    private final PaymentTransactionRepository transactionRepository;
    private final RefundTransactionRepository refundRepository;
    private final PaymentOutboxWriter outboxWriter;
    private final PaymentDomainService domainService;
    private final PaymentClient paymentClient;
    private final PayUniConfig payUniConfig;
    private final OrderGrpcClient orderGrpcClient;
    private final PaymentLogMapper paymentLogMapper;
    private final AtmReactivationService atmReactivationService;
    private final PaymentCloseService paymentCloseService;

    /** ATM 預設有效天數（由 application.yml payment.default-expire-days 設定） */
    @Value("${payment.default-expire-days:1}")
    private int defaultExpireDays;

    public PaymentApplicationService(
            PaymentTransactionRepository transactionRepository,
            RefundTransactionRepository refundRepository,
            PaymentOutboxWriter outboxWriter,
            PaymentDomainService domainService,
            PaymentClient paymentClient,
            PayUniConfig payUniConfig,
            OrderGrpcClient orderGrpcClient,
            PaymentLogMapper paymentLogMapper,
            AtmReactivationService atmReactivationService,
            PaymentCloseService paymentCloseService
    ) {
        this.transactionRepository = transactionRepository;
        this.refundRepository = refundRepository;
        this.outboxWriter = outboxWriter;
        this.domainService = domainService;
        this.paymentClient = paymentClient;
        this.payUniConfig = payUniConfig;
        this.orderGrpcClient = orderGrpcClient;
        this.paymentLogMapper = paymentLogMapper;
        this.atmReactivationService = atmReactivationService;
        this.paymentCloseService = paymentCloseService;
    }

    /**
     * §2A.3 切換流程：將舊 row 標為 CLOSED_BY_SWITCH，新 row 由呼叫端建立。
     * <p>
     * 不主動關閉 PayUni 端舊交易（C2 約束 + §2A.6 兜底）。
     */
    private void handleSwitch(PaymentTransaction oldRow) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        int updated = transactionRepository.markClosedBySwitch(oldRow.id(), now, oldRow.version());
        if (updated == 0) {
            log.warn("PaymentApplicationService.handleSwitch.raceLost",
                    "舊 row 標 CLOSED_BY_SWITCH 失敗（樂觀鎖或狀態變更）trade_no=" + oldRow.tradeNo());
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ALREADY_EXISTS);
        }
        log.info("PaymentApplicationService.handleSwitch.closed",
                "舊 row 標 CLOSED_BY_SWITCH order_no=" + oldRow.orderId()
                        + " old_type=" + oldRow.paymentType().value()
                        + " trade_no=" + oldRow.tradeNo());
    }

    /**
     * 建立支付交易
     *
     * 冪等性設計（三層縱深防禦）：
     * 1. 應用層：查詢 order_id 是否已有活躍交易（INIT 或 PAID）
     *    - PAID → 拋出異常，禁止重複扣款
     *    - INIT → 回傳既有交易資訊（不重新呼叫 PayUni）
     * 2. DB 層：partial unique index (order_id WHERE status IN (0,1)) 作為最後防線
     * 3. PayUni 層：MerTradeNo（trade_no）去重（同一 trade_no 不會重複扣款）
     */
    @Transactional
    public CreatePaymentResponse createPayment(CreatePaymentRequest request) {
        // 0. 向 Order Service 查詢訂單付款資訊
        GetOrderForPaymentResponse orderResponse = orderGrpcClient.getOrderForPayment(request.orderNo());
        if (orderResponse.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new PaymentBusinessException(GrpcResponseCode.ORDER_QUERY_FAILED, orderResponse.getMessage());
        }

        OrderPaymentInfo orderInfo = orderResponse.getData();

        // 1. 驗證買家身分（防止非本人付款，防護 Proto 空值）
        //    兩邊都是已解析的 buyer_id：orderInfo.getBuyerId() 來自 order-service 的
        //    OrderPaymentInfo.buyer_id，request.buyerId() 來自 gateway 已解析後注入的
        //    CreatePaymentRequest.buyer_id（見 PaymentGrpcService.createPayment 的註解）。
        String orderBuyerIdStr = orderInfo.getBuyerId();
        InputValidator.requireNotEmpty(orderBuyerIdStr, "買家帳號");
        long orderBuyerId = Long.parseLong(orderBuyerIdStr);
        if (orderBuyerId != request.buyerId()) {
            throw new PaymentBusinessException(GrpcResponseCode.NO_PERMISSION_FOR_PAYMENT);
        }

        // 2. 驗證訂單狀態（僅待付款訂單可建立支付）
        if (!OrderStatus.PENDING_PAYMENT.value().equals(orderInfo.getStatus())) {
            throw new PaymentBusinessException(GrpcResponseCode.ORDER_STATUS_NOT_PAYABLE,
                    OrderStatus.fromValue(orderInfo.getStatus()).label());
        }

        // 3. 從訂單資訊取得付款參數（防護 Proto 空值）
        String merId = orderInfo.getMerId();
        String amountStr = orderInfo.getAmount();
        InputValidator.requireNotEmpty(amountStr, "付款金額");
        long amount;
        try {
            amount = Long.parseLong(amountStr.trim());
        } catch (NumberFormatException e) {
            throw new PaymentBusinessException(GrpcResponseCode.INVALID_PARAM, amountStr);
        }
        if (amount < 0) {
            throw new PaymentBusinessException(GrpcResponseCode.INVALID_PARAM, String.valueOf(amount));
        }
        String productName = orderInfo.getProductName();
        InputValidator.requireNotEmpty(merId, "商家編號");

        // §5.9 解析 client_type / app_type（空字串視為預設值）
        String clientType = resolveClientType(request.clientType());
        String appType = resolveAppType(request.appType());

        // 3A. §2A.1 零元訂單：跳過所有後續金額/狀態防護，直接 PAID
        if (amount == 0) {
            return handleZeroAmount(request, merId, productName, clientType, appType);
        }

        // 3B. §2A.2 按 paymentMethod 精準上下限檢核
        //  - 信用卡/Apple Pay/Google Pay: 1~199,999
        //  - ATM: 15~49,999
        try {
            PaymentMethodLimitRules.validateOrThrow(request.paymentType(), amount);
        } catch (PaymentMethodLimitRules.OutOfRangeException e) {
            log.warn("PaymentApplicationService.createPayment.amountOutOfRange",
                    "付款方式金額超出範圍 order_no=" + request.orderNo()
                            + " method=" + e.method() + " amount=" + e.amount()
                            + " min=" + e.min() + " max=" + e.max());
            throw new PaymentBusinessException(
                    GrpcResponseCode.PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE,
                    e.label(), e.amount(), e.min(), e.max());
        }

        // 4. §2A.3 切換偵測：先看同 type 是否有 row
        Optional<PaymentTransaction> sameTypeOpt = transactionRepository.findActiveByOrderIdAndPaymentType(
                request.orderNo(), request.paymentType());
        if (sameTypeOpt.isPresent()) {
            PaymentTransaction sameRow = sameTypeOpt.get();
            // §2A.3.3 / §2A.5.3 L2 即時過期檢查：所有 type 統一用 expire_at 判定
            if (sameRow.status() == PaymentTransactionStatus.INIT
                    && sameRow.expireAt() != null
                    && DateTimeUtils.nowUtc().isAfter(sameRow.expireAt())) {
                int updated = transactionRepository.updateToExpired(sameRow.tradeNo(), sameRow.version());
                log.warn("PaymentApplicationService.createPayment.expiredL2",
                        "row 已過期，L2 即時標 EXPIRED trade_no=" + sameRow.tradeNo() + " updated=" + updated);
                // fall through 到 4A 走切換/新建流程
            } else {
                // 同 paymentType 重打，未過期 → 冪等回 cached
                return handleExistingTransaction(sameRow);
            }
        }

        // 4A. 同 type 無 → 看是否有不同 type 的 row（切換情境）
        Optional<PaymentTransaction> otherTypeOpt = transactionRepository.findActiveByOrderId(request.orderNo());
        if (otherTypeOpt.isPresent()) {
            PaymentTransaction other = otherTypeOpt.get();
            // 已 PAID → 禁止重複扣款
            if (other.status() == PaymentTransactionStatus.PAID) {
                throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ALREADY_EXISTS);
            }

            // §2A.5.4 ATM 復活分支：切回 ATM 時優先嘗試復活舊 ATM CLOSED_BY_SWITCH row，
            // 不打 PayUni、不發新虛擬帳號。
            if (request.paymentType() == PaymentMethod.ATM) {
                Optional<CreatePaymentResponse> reactivated =
                        atmReactivationService.tryReactivate(request, amount);
                if (reactivated.isPresent()) {
                    return reactivated.get();
                }
                // 無可復活 row → fall through 到切換 + 新建流程
            }

            // §2A.3 切換流程：標舊 row 為 CLOSED_BY_SWITCH（MerTradeNo 序列化後無需冷卻）
            handleSwitch(other);
        }

        // 5. 建立交易記錄（§2A.3.6 MerTradeNo 序列化：trade_no = orderNo + attemptSeq）
        int nextSeq = transactionRepository.findMaxAttemptSeqByOrderId(request.orderNo())
                .map(seq -> seq + 1)
                .orElse(1);

        // §2A.4：信用卡/Apple Pay/Google Pay 設 expire_at = now + 600s（與 PayUni TradeLExpireSec=600 同步）
        // ATM 的 expire_at 由 PayUni /api/atm response 的 ExpireDate 後續更新
        LocalDateTime expireAt = null;
        if (request.paymentType() == PaymentMethod.CREDIT_CARD
                || request.paymentType() == PaymentMethod.APPLE_PAY
                || request.paymentType() == PaymentMethod.GOOGLE_PAY) {
            expireAt = DateTimeUtils.nowUtc().plusSeconds(PaymentExpiryConstants.CREDIT_CARD_EXPIRE_SECONDS);
        }

        PaymentTransaction transaction = PaymentTransaction.create(
                request.orderNo(),
                nextSeq,
                merId,
                String.valueOf(request.buyerId()),
                request.paymentType(),
                amount,
                productName,
                expireAt,
                clientType,
                appType
        );

        transactionRepository.save(transaction);
        // §2A.7 訂單付款方式快照同步（觸發點 #1 / #2）
        orderGrpcClient.snapshotPaymentInfo(
                request.orderNo(), transaction.tradeNo(),
                transaction.paymentType().value(), null, null, null);
        log.info("PaymentApplicationService.create.success", "支付交易建立成功", "trade_no", transaction.tradeNo());

        // 6. 根據支付方式呼叫 PayUni
        return switch (request.paymentType()) {
            case CREDIT_CARD -> handleCreditCardPayment(transaction);
            case APPLE_PAY -> handleApplePayPayment(transaction, request.applePayToken(), request.userEmail());
            case GOOGLE_PAY -> handleGooglePayPayment(transaction, request.googlePayToken(), request.userEmail());
            case ATM -> handleAtmPayment(transaction, resolveExpireDays(request.expireDays()), request.getBankType(), request.userEmail());
            default -> throw new PaymentBusinessException(GrpcResponseCode.INVALID_PAYMENT_TYPE,
                    unsupportedPaymentMethodLabel(request.paymentType()));
        };
    }

    /**
     * §2A.1 零元訂單處理：全額折抵後無需付款。
     * <p>
     * 不呼叫任何 PayUni API，直接寫入 payment_transactions（status=PAID, paymentType=ZERO_AMOUNT）
     * 並發布 payment.paid event，下游（出貨 / 核銷券 / 扣庫存）行為與一般付款完成一致。
     * <p>
     * 冪等：若同一 order 已有 ZERO_AMOUNT 活躍交易，直接回傳既有 row。
     */
    private CreatePaymentResponse handleZeroAmount(CreatePaymentRequest request, String merId, String productName,
                                                    String clientType, String appType) {
        // 冪等檢查：若同一訂單已存在 ZERO_AMOUNT PAID row，直接回傳（避免重複寫）
        Optional<PaymentTransaction> existingOpt = transactionRepository.findActiveByOrderId(request.orderNo());
        if (existingOpt.isPresent()) {
            PaymentTransaction existing = existingOpt.get();
            if (existing.paymentType() == PaymentMethod.ZERO_AMOUNT) {
                log.info("PaymentApplicationService.createPayment.zeroAmount.idempotent",
                        "零元訂單已存在 PAID row，回傳既有 order_no=" + request.orderNo() + " trade_no=" + existing.tradeNo());
                return CreatePaymentResponse.forZeroAmount(existing.tradeNo(), existing.orderId());
            }
            // 其他情況（使用者原本選了某 paymentType，後來金額變 0）：沿用 §2A.3 切換規則在 P5 處理
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ALREADY_EXISTS);
        }

        int nextSeq = transactionRepository.findMaxAttemptSeqByOrderId(request.orderNo())
                .map(seq -> seq + 1)
                .orElse(1);
        PaymentTransaction transaction = PaymentTransaction.createZeroAmount(
                request.orderNo(), nextSeq, merId, String.valueOf(request.buyerId()), productName,
                clientType, appType);
        transactionRepository.save(transaction);
        log.info("PaymentApplicationService.createPayment.zeroAmount.success",
                "零元訂單直接以 PAID 落地 order_no=" + request.orderNo() + " trade_no=" + transaction.tradeNo());

        // §2A.7 訂單付款方式快照同步（觸發點 #1）：寫入 orders.payment_method = 2900
        // ATM 欄位皆為 null（零元訂單不會有 ATM 取號）
        orderGrpcClient.snapshotPaymentInfo(
                request.orderNo(), transaction.tradeNo(),
                PaymentMethod.ZERO_AMOUNT.value(), null, null, null);

        // 發布 payment.paid event，讓下游照常出貨 / 核銷券 / 扣庫存
        PaymentDomainEvent.PaymentSucceeded event = domainService.createPaymentSucceededEvent(
                transaction, DateTimeUtils.nowUtc());
        publishEvent(event);

        return CreatePaymentResponse.forZeroAmount(transaction.tradeNo(), transaction.orderId());
    }

    /**
     * 處理已存在活躍交易的情況（冪等性回應）
     *
     * - PAID：訂單已付款，禁止重複建立
     * - INIT：回傳既有交易資訊，不重新呼叫 PayUni
     *   - 信用卡：重新產生 UPP Form（同一 trade_no，PayUni 冪等）
     *   - ATM：回傳 DB 中已儲存的虛擬帳號
     *   - Apple/Google Pay：回傳既有交易狀態
     */
    private CreatePaymentResponse handleExistingTransaction(PaymentTransaction existing) {
        if (existing.status() == PaymentTransactionStatus.PAID) {
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ALREADY_EXISTS);
        }

        // INIT 狀態：回傳既有交易的冪等性回應
        log.info("PaymentApplicationService.create.idempotent", "訂單已有進行中的交易，回傳既有資料", "trade_no", existing.tradeNo());

        return switch (existing.paymentType()) {
            // 信用卡：重新產生 UPP Form（同一 trade_no，PayUni 以 MerTradeNo 去重）
            case CREDIT_CARD -> handleCreditCardPayment(existing);

            // ATM：回傳 DB 中已儲存的虛擬帳號資訊
            case ATM -> CreatePaymentResponse.forAtm(
                    existing.tradeNo(), existing.orderId(), existing.amount(),
                    existing.atmBankCode(), existing.atmVirtualAccount(),
                    existing.atmExpireDate(), existing.expireAt()
            );

            // Apple Pay / Google Pay：回傳既有交易狀態
            case APPLE_PAY -> CreatePaymentResponse.forApplePay(
                    existing.tradeNo(), existing.orderId(), existing.amount(),
                    existing.status(), null
            );
            case GOOGLE_PAY -> CreatePaymentResponse.forGooglePay(
                    existing.tradeNo(), existing.orderId(), existing.amount(),
                    existing.status(), null
            );

            default -> throw new PaymentBusinessException(GrpcResponseCode.INVALID_PAYMENT_TYPE,
                    unsupportedPaymentMethodLabel(existing.paymentType()));
        };
    }

    private String unsupportedPaymentMethodLabel(PaymentMethod paymentMethod) {
        return paymentMethod != null ? paymentMethod.label() : "未知支付方式";
    }

    /**
     * 查詢支付詳情
     */
    public Optional<PaymentDetailResponse> getPaymentDetail(String tradeNo) {
        // §2A.6 後同 trade_no 可能多 row，必須走 canonical 取「目前生效的主 row」
        return transactionRepository.findCanonicalByTradeNo(tradeNo)
                .map(PaymentDetailResponse::from);
    }

    /**
     * §2A.3.6 / §2A.6 查詢同 trade_no（= order_no）的所有歷史 row。
     * <p>
     * 含 CLOSED_BY_SWITCH（切換而關閉的舊 row）與重複付款的副 row（is_duplicate=true）。
     * Repository 端 ORDER BY created_at DESC，最新的在前。
     */
    public java.util.List<PaymentDetailResponse> getPaymentHistory(String orderId) {
        return transactionRepository.findHistoryByOrderId(orderId)
                .stream()
                .map(PaymentDetailResponse::from)
                .toList();
    }

    /**
     * #2501 由 canonical order 反解該訂單的賣家 store_id，供 seller 付款歷程 store 歸屬驗證。
     *
     * <p>payment_transactions 本身不存 store_id，店家歸屬只能由 order-service 的
     * canonical order 反解。回傳 0 代表訂單不存在、order-service 未提供 store_id 或值非法
     * ——呼叫端一律把 0 當「查無」，與跨店越權回同形 NOT_FOUND（防枚舉）。此處不拋業務例外，
     * 授權判斷集中在 gRPC adapter（{@code PaymentGrpcService.getSellerPaymentHistory}）。
     *
     * @param orderNo 原始訂單編號
     * @return 該訂單的賣家 store_id；查無或缺值時為 0
     */
    public long resolveOrderStoreId(String orderNo) {
        GetOrderForPaymentResponse orderResponse = orderGrpcClient.getOrderForPayment(orderNo);
        if (orderResponse.getCode() != GrpcResponseCode.SUCCESS.code() || !orderResponse.hasData()) {
            return 0L;
        }
        String storeIdStr = orderResponse.getData().getStoreId();
        if (storeIdStr == null || storeIdStr.isBlank()) {
            return 0L;
        }
        try {
            return Long.parseLong(storeIdStr.trim());
        } catch (NumberFormatException e) {
            log.warn("PaymentApplicationService.resolveOrderStoreId.badStoreId",
                    "order-service 回傳非數字 store_id order_no=" + orderNo + " store_id=" + storeIdStr);
            return 0L;
        }
    }

    /**
     * 組裝 ATM 退款頁面版表單資料（頁面版 — 買家自行填寫銀行資訊）
     *
     * @param merId           子商家 MerID
     * @param providerTradeNo PayUni UNi 序號
     * @param amount          退款金額
     * @param returnUrl       完成後返回 URL
     * @return UppFormData（由呼叫端透過 PaymentPageService 轉為 redirect URL）
     */
    public UppFormData buildRefundPageFormData(String merId, String providerTradeNo, long amount, String returnUrl) {
        return paymentClient.offlineRefundPage(merId, providerTradeNo, amount, returnUrl);
    }

    /**
     * 同步查詢支付狀態（呼叫 PayUni Query API）
     */
    @Transactional
    public Optional<PaymentDetailResponse> syncPaymentStatus(String tradeNo) {
        Optional<PaymentTransaction> optTransaction = transactionRepository.findCanonicalByTradeNo(tradeNo);
        if (optTransaction.isEmpty()) {
            return Optional.empty();
        }

        PaymentTransaction transaction = optTransaction.get();

        // 已是終態，直接返回
        if (PaymentStatusRules.isTerminal(transaction.status())) {
            return Optional.of(PaymentDetailResponse.from(transaction));
        }

        // 呼叫 PayUni 查詢 API
        PayUniApiResponse response = paymentClient.queryTransaction(transaction.merId(), tradeNo);
        if (!response.isSuccess()) {
            log.warn("PaymentApplicationService.query.failed", "PayUni 查詢失敗: " + response.getErrorMessage(), "trade_no", tradeNo);
            return Optional.of(PaymentDetailResponse.from(transaction));
        }

        // 解析狀態並更新
        String providerStatus = response.getTradeStatus();
        Optional<PaymentTransactionStatus> statusOpt = domainService.parseProviderStatus(providerStatus);

        if (statusOpt.isPresent()) {
            PaymentTransactionStatus newStatus = statusOpt.get();
            updateTransactionStatus(transaction, newStatus, response);
        }

        // 重新查詢並返回
        return transactionRepository.findCanonicalByTradeNo(tradeNo)
                .map(PaymentDetailResponse::from);
    }

    /**
     * 依訂單編號關閉付款交易（Order Service 取消訂單時呼叫）
     * <p>
     * 冪等：無活躍交易、已非 INIT 狀態皆回傳 true。
     * <p>
     * §5.4 Tx 載體規定：本方法**不**標 {@code @Transactional}（僅做 SELECT + 委派，無多步原子寫入需求），
     * 實際 tx 由 {@link PaymentCloseService#closePayment} 透過 cross-bean proxy 真實生效。
     */
    public boolean closePaymentByOrderNo(String orderNo) {
        Optional<PaymentTransaction> optTransaction = transactionRepository.findActiveByOrderId(orderNo);
        if (optTransaction.isEmpty()) {
            log.info("PaymentApplicationService.closeByOrderNo.noActive", "無活躍交易，視為成功", "orderNo", orderNo);
            return true;
        }

        PaymentTransaction transaction = optTransaction.get();
        if (!domainService.canClose(transaction)) {
            log.info("PaymentApplicationService.closeByOrderNo.skip", "交易已非 INIT，視為成功，當前狀態: " + transaction.status(), "orderNo", orderNo);
            return true;
        }

        boolean closed = paymentCloseService.closePayment(transaction.tradeNo());
        if (closed) {
            log.info("PaymentApplicationService.closeByOrderNo.success", "訂單付款交易已關閉: orderNo=" + orderNo + ", tradeNo=" + transaction.tradeNo());
        }
        return closed;
    }

    /**
     * 發動撥款指示（代理商專用）— 收斂規範 §8.1 同步 retry 最多一次，
     * 成功/失敗皆寫入 payment_transactions 的 granted_at / grant_status / grant_message
     * （SoT，收斂規範 §6.2 撥款欄位封閉；僅 Admin API 可暴露）。
     *
     * 由 Order Service GrantPaymentWorker 在買家確認收貨後呼叫。
     *
     * @param orderNo 訂單編號
     * @return 撥款結果
     */
    @Transactional
    public GrantPaymentResult grantPayment(String orderNo) {
        InputValidator.requireNotEmpty(orderNo, "訂單編號");

        // §5.6 查詢並鎖定該訂單的已付款交易（FOR UPDATE）：行鎖持有至事務結束（跨 PayUni Grant 呼叫），
        // 序列化同交易的併發 / 重試撥款。撥款無「每次嘗試中間狀態」，必須持鎖跨外部呼叫才能讓第二次
        // 撥款取鎖後看到 grant_status='SUCCESS' 而短路，杜絕時序型雙重撥款。
        Optional<PaymentTransaction> optTransaction = transactionRepository.findActiveByOrderIdForUpdate(orderNo);
        if (optTransaction.isEmpty()) {
            throw new PaymentBusinessException(GrpcResponseCode.TRANSACTION_NOT_FOUND_FOR_ORDER);
        }

        PaymentTransaction transaction = optTransaction.get();

        // §5.6 撥款前冪等短路：鎖後檢查 grant_status，已成功則直接回傳既有結果，不再打 PayUni（避免重撥）
        if (PayUniConstants.RESPONSE_SUCCESS.equals(transaction.grantStatus())) {
            log.info("PaymentApplicationService.grant.idempotent",
                    "撥款已成功，冪等短路不重打 PayUni",
                    "trade_no", transaction.tradeNo());
            return grantResult(transaction, transaction.grantedAt(),
                    PayUniConstants.RESPONSE_SUCCESS, transaction.grantMessage(),
                    GrantResponseRules.GRANT_OUTCOME_GRANTED);
        }

        // 驗證：必須為 PAID 狀態（FOR UPDATE 已限定 status='1'，此為縱深防禦）
        if (transaction.status() != PaymentTransactionStatus.PAID) {
            throw new PaymentBusinessException(GrpcResponseCode.TRANSACTION_NOT_PAID, transaction.status().label());
        }

        // 驗證：必須有 PayUni UNi序號
        InputValidator.requireNotEmpty(transaction.providerTradeNo(), "金流交易編號");

        // §5.6 去重硬化 B：非首撥（grant_status 非 null，即曾嘗試並落地 PENDING_CONFIRM / PayUni 錯誤碼）
        // → 重發前先 grant_query 對帳，避免對「已入帳」交易重複撥款。首撥（grant_status==null）不查，直接發動。
        if (transaction.grantStatus() != null) {
            ReconcileOutcome rec = reconcileGrantViaQuery(transaction);
            switch (rec) {
                case HIT_ALREADY_GRANTED -> {
                    // grant_query 命中已入帳 → 補記 SUCCESS 終態，不重發、不打 Grant API
                    LocalDateTime now = DateTimeUtils.nowUtc();
                    String msg = "grant_query 對帳確認已入帳，補記終態";
                    transactionRepository.updateGrantResult(
                            transaction.id(), now, PayUniConstants.RESPONSE_SUCCESS, msg);
                    log.info("PaymentApplicationService.grant.reconcile.hit",
                            "grant_query 對帳命中已入帳，標記 SUCCESS 不重發",
                            "trade_no", transaction.tradeNo());
                    insertGrantLog(transaction.tradeNo(), "INFO", "GRANT_RECONCILED", msg);
                    return grantResult(transaction, now, PayUniConstants.RESPONSE_SUCCESS, msg,
                            GrantResponseRules.GRANT_OUTCOME_GRANTED);
                }
                case INCONCLUSIVE -> {
                    // 未查完 / 落在入帳前 / 超出 30 天窗口 → 維持既有非成功終態，不重發、不打 Grant API。
                    // 刻意不改寫 grant_status / updated_at，保留原對帳區間錨點（下輪隨 today 前進再對帳）。
                    log.warn("PaymentApplicationService.grant.reconcile.inconclusive",
                            "grant_query 對帳未完成/落在入帳前，維持待確認、不重發",
                            "trade_no", transaction.tradeNo());
                    insertGrantLog(transaction.tradeNo(), "WARNING", "GRANT_PENDING_CONFIRM",
                            "grant_query 對帳未完成，維持待確認、不重發");
                    String msg = transaction.grantMessage() != null
                            ? transaction.grantMessage() : "grant_query 對帳未完成，維持待確認";
                    return grantResult(transaction, transaction.grantedAt(),
                            transaction.grantStatus(), msg, null);
                }
                case CONFIRMED_NOT_GRANTED -> {
                    // 全區間 + 全分頁查完仍查無 → 確認未撥，續行安全重發（落入下方 invokeGrantWithRetry）
                    log.info("PaymentApplicationService.grant.reconcile.notGranted",
                            "grant_query 對帳確認未撥款，安全重發",
                            "trade_no", transaction.tradeNo());
                }
            }
        }

        // 首撥 或 對帳確認未撥 → 發動 Grant（invokeGrantWithRetry try/catch 捕捉 PayUniApiException，不穿透）
        GrantOutcome outcome = invokeGrantWithRetry(transaction);
        LocalDateTime now = DateTimeUtils.nowUtc();

        if (outcome.terminal() == GrantTerminal.SUCCESS) {
            transactionRepository.updateGrantResult(
                    transaction.id(), now, PayUniConstants.RESPONSE_SUCCESS, outcome.message());
            log.info("PaymentApplicationService.grant.success",
                    "撥款指示成功, amount=" + transaction.amount(),
                    "trade_no", transaction.tradeNo());
            insertGrantLog(transaction.tradeNo(), "INFO", "GRANT_SUCCESS",
                    "撥款指示成功, amount=" + transaction.amount());
            return grantResult(transaction, now, PayUniConstants.RESPONSE_SUCCESS, outcome.message(),
                    GrantResponseRules.GRANT_OUTCOME_GRANTED);
        }

        if (outcome.terminal() == GrantTerminal.PROVIDER_TOOK_OVER) {
            // §5.6.1：PayUni 已對退款交易接管撥款——預期收斂結果非錯誤。不重試、不進 PENDING_CONFIRM
            // 對帳迴圈；grant_status 落 PayUni 回應碼原樣（隨本交易 commit 存活）、granted_at 刻意保持
            // NULL（帳務權威 = PayUni 端明細 + payment_refunds / financial_logs）。以「正常回應」帶
            // grant_outcome=PROVIDER_TOOK_OVER 回傳，Order 端 Worker 據此收斂 task 為 PROVIDER_AUTO（03 §3.11.2）。
            transactionRepository.updateGrantFailure(
                    transaction.id(), outcome.grantStatus(), outcome.message(), now);
            log.info("PaymentApplicationService.grant.providerTookOver",
                    "撥款遭拒且該交易存在已完成退款，PayUni 已接管撥款（不重試）: status="
                            + outcome.grantStatus() + ", msg=" + outcome.message()
                            + ", provider_trade_no=" + transaction.providerTradeNo(),
                    "trade_no", transaction.tradeNo());
            insertGrantLog(transaction.tradeNo(), "INFO", "GRANT_PROVIDER_TOOK_OVER",
                    "PayUni 已接管撥款（退款交易）: " + outcome.grantStatus() + " / " + outcome.message());
            return grantResult(transaction, null, outcome.grantStatus(), outcome.message(),
                    GrantResponseRules.GRANT_OUTCOME_PROVIDER_TOOK_OVER);
        }

        // 失敗終態（PENDING_CONFIRM 或 PayUni 錯誤碼）：write-then-return —— 隨交易 commit 存活、不 throw
        // rollback（比照 RefundApplicationService.handleRefundFailure）。granted_at 保持 null（僅 SUCCESS 寫入）。
        // 失敗訊號改由 gRPC adapter 於交易 commit 後依已落地的 grant_status != SUCCESS 產生（§5.6 P0-3a）。
        transactionRepository.updateGrantFailure(
                transaction.id(), outcome.grantStatus(), outcome.message(), now);
        boolean ambiguous = outcome.terminal() == GrantTerminal.PENDING_CONFIRM;
        log.warn("PaymentApplicationService.grant.failed",
                "撥款未成功（" + (ambiguous ? "模稜兩可，標 PENDING_CONFIRM" : "明確未發動，retry 後仍失敗")
                        + "）: status=" + outcome.grantStatus() + ", msg=" + outcome.message()
                        + ", provider_trade_no=" + transaction.providerTradeNo(),
                "trade_no", transaction.tradeNo());
        insertGrantLog(transaction.tradeNo(),
                ambiguous ? "WARNING" : "ERROR",
                ambiguous ? "GRANT_PENDING_CONFIRM" : "GRANT_FAILED",
                "撥款未成功: " + outcome.grantStatus() + " / " + outcome.message());
        // granted_at = null（失敗路徑不寫里程碑）；grantOutcome=null → adapter 依 grant_status 轉 gRPC error
        return grantResult(transaction, null, outcome.grantStatus(), outcome.message(), null);
    }

    /** 組裝撥款結果 DTO（grantStatus/grantMessage 供 adapter 判斷成敗、grantOutcome 供組 grant_outcome；§5.6 P0-3a / §5.6.1）。 */
    private GrantPaymentResult grantResult(PaymentTransaction tx, LocalDateTime grantedAt,
                                           String grantStatus, String grantMessage, String grantOutcome) {
        return new GrantPaymentResult(tx.tradeNo(), tx.orderId(), tx.providerTradeNo(),
                tx.amount(), grantedAt, grantStatus, grantMessage, grantOutcome);
    }

    /** 撥款單次呼叫的終態分類（§5.6 去重硬化 A + §5.6.1 退款接管）。 */
    private enum GrantTerminal { SUCCESS, FAILED_CODE, PENDING_CONFIRM, PROVIDER_TOOK_OVER }

    /**
     * 撥款呼叫結果封裝。
     *
     * @param terminal    終態分類
     * @param grantStatus 落地的 grant_status（SUCCESS / PayUni 錯誤碼 / PENDING_CONFIRM）
     * @param message     grant_message（成功文案 / PayUni 訊息 / 模稜兩可原因）
     */
    private record GrantOutcome(GrantTerminal terminal, String grantStatus, String message) {}

    /**
     * 呼叫 PayUni Grant API（§5.6 去重硬化 A / §5.6.1 退款接管 / 收斂規範 §8.1）。
     * <p>
     * try/catch 捕捉 {@code PayUniApiException}（不穿透，否則寫入根本不執行）；連同非成功回應交
     * {@link GrantResponseRules#classifyFailure} 判類：
     * <ul>
     *   <li>SUCCESS → 立即返回</li>
     *   <li>明確未發動（DEFINITELY_NOT_SENT）→ 同步 retry <b>最多一次</b>（PayUni 未受理，可安全重試）</li>
     *   <li>模稜兩可（AMBIGUOUS：transport 失敗 / 不可信回應）→ <b>不自動重試</b>，標 PENDING_CONFIRM</li>
     *   <li>已由 PayUni 接管（PROVIDER_TOOK_OVER：可信拒絕 + 存在已完成退款，§5.6.1）→
     *       <b>不重試</b>，grant_status 落 PayUni 原碼、以正常回應帶 grant_outcome</li>
     * </ul>
     * 接管主判準（存在已完成退款）採<b>延遲查詢</b>：僅在 classifyFailure 遇「確定拒絕候選碼」
     * 時才查 payment_refunds（BooleanSupplier），成功 / 模稜兩可路徑不查。
     */
    private GrantOutcome invokeGrantWithRetry(PaymentTransaction transaction) {
        GrantOutcome first = singleGrant(transaction);
        // 模稜兩可 / 接管 / 成功 → 不重試（模稜兩可為 0 次 retry，仍在「最多一次」語意內；接管重試必然同碼）
        if (first.terminal() != GrantTerminal.FAILED_CODE) {
            return first;
        }
        // 明確未發動 → 安全 retry 一次
        log.warn("PaymentApplicationService.grant.retry",
                "Grant 首次明確未發動，retry 一次: " + first.grantStatus() + " / " + first.message(),
                "trade_no", transaction.tradeNo());
        return singleGrant(transaction);
    }

    /** 單次 Grant 呼叫 + 分類（try/catch 捕捉 PayUniApiException 不穿透）。 */
    private GrantOutcome singleGrant(PaymentTransaction transaction) {
        try {
            PayUniApiResponse response = paymentClient.grant(
                    payUniConfig.getAgentId(), transaction.merId(),
                    transaction.providerTradeNo(), transaction.amount());
            if (response.isSuccess()) {
                String msg = response.getErrorMessage();
                if (msg == null || msg.isBlank()) {
                    msg = "撥款指示成功";
                }
                return new GrantOutcome(GrantTerminal.SUCCESS, PayUniConstants.RESPONSE_SUCCESS, msg);
            }
            String code = response.status();
            String msg = response.getErrorMessage();
            // §5.6.1 延遲判準：supplier 僅在 code 為「確定拒絕候選碼」（非 AMBIGUOUS）時被評估
            return switch (GrantResponseRules.classifyFailure(code,
                    () -> refundRepository.hasSucceededRefund(transaction.tradeNo()))) {
                case AMBIGUOUS -> new GrantOutcome(GrantTerminal.PENDING_CONFIRM, PayUniConstants.PENDING_CONFIRM,
                        "撥款回應模稜兩可(" + code + "): " + (msg != null ? msg : ""));
                // §5.6.1：grant_status 落 PayUni 回應碼原樣（非哨兵值）；message 帶原碼供 Order 端 lastError 稽核
                case PROVIDER_TOOK_OVER -> new GrantOutcome(GrantTerminal.PROVIDER_TOOK_OVER, code,
                        "PayUni 已接管撥款（退款交易，拒絕補發 " + code + "）: " + (msg != null ? msg : ""));
                case DEFINITELY_NOT_SENT -> new GrantOutcome(GrantTerminal.FAILED_CODE, code,
                        msg != null ? msg : "撥款遭 PayUni 拒絕");
            };
        } catch (PayUniApiException e) {
            // transport 失敗（timeout / IO / HTTP_ERROR / EMPTY_RESPONSE）→ 模稜兩可（PayUni 可能已收到指示）→ 不重試
            log.warn("PaymentApplicationService.grant.ambiguousException",
                    "Grant 呼叫模稜兩可例外: " + e.getErrorCode() + " / " + e.getMessage(),
                    "trade_no", transaction.tradeNo());
            return new GrantOutcome(GrantTerminal.PENDING_CONFIRM, PayUniConstants.PENDING_CONFIRM,
                    "撥款呼叫例外(" + e.getErrorCode() + "): " + e.getMessage());
        }
    }

    /** §5.6 去重硬化 B：grant_query 對帳結果。 */
    private enum ReconcileOutcome { HIT_ALREADY_GRANTED, CONFIRMED_NOT_GRANTED, INCONCLUSIVE }

    /**
     * §5.6 去重硬化 B：重發前 grant_query 對帳。
     * <p>
     * 逐日涵蓋 {@code [撥款嘗試日, 今日]}（撥款嘗試日 = 交易 updated_at 換算台北日曆日；受 grant_query
     * 30 天窗口約束）、各日遍歷全分頁至 {@code PageInfo.Total}，以 provider_trade_no 精確比對是否已入帳。
     * <ul>
     *   <li>命中 → {@code HIT_ALREADY_GRANTED}（標 SUCCESS、不重發）</li>
     *   <li>全區間逐日（含 {@code QUERY03002} 有效空日）+ 全分頁查完仍查無 → {@code CONFIRMED_NOT_GRANTED}（安全重發）</li>
     *   <li>落在入帳前（撥款隔日入帳，today 未越過嘗試日）/ 超出 30 天窗口 / 該日未查完
     *       （{@code QUERY02004} 超窗 / transport 錯誤 / 其他非預期 Status）→ {@code INCONCLUSIVE}（維持 PENDING_CONFIRM、不盲目重發）</li>
     * </ul>
     * <p>
     * <b>issue #1329</b>：grant_query 對「窗口內某日無撥款」回 {@code QUERY03002}（查無符合撥款資料）——這是<b>有效空頁</b>
     * （撥款隔日入帳，嘗試日當天恆空為常態），視為該日 count=0 續掃下一日，<b>不</b>算查詢失敗；
     * 唯有 {@code QUERY02004}（超 30 天窗）/ transport / 其他非預期 Status 才算「未查完 → INCONCLUSIVE」
     * （分類見 {@link GrantResponseRules#isGrantQueryDayComplete}）。
     * 全程在持鎖的同一 {@code @Transactional} 寫入路徑內同步執行（§22：不引入獨立 reconciliation job）。
     */
    private ReconcileOutcome reconcileGrantViaQuery(PaymentTransaction transaction) {
        String providerTradeNo = transaction.providerTradeNo();
        LocalDate today = LocalDate.now(PAYUNI_ZONE);

        // 撥款嘗試日：交易 updated_at（UTC 值語意）→ 台北日曆日。updated_at 於每次撥款終態寫入時更新，
        // 為對帳區間有效起點（設計書 B P1-1：不可用 granted_at，其失敗路徑為 null）。
        LocalDateTime updatedAt = transaction.updatedAt() != null
                ? transaction.updatedAt() : DateTimeUtils.nowUtc();
        LocalDate attemptDay = updatedAt.atOffset(ZoneOffset.UTC)
                .atZoneSameInstant(PAYUNI_ZONE).toLocalDate();

        // 落在入帳前：撥款「隔日」入帳，today 未越過嘗試日 → 尚不能斷定未撥 → INCONCLUSIVE（不重發）。
        if (!today.isAfter(attemptDay)) {
            log.info("PaymentApplicationService.grant.reconcile.beforePosting",
                    "對帳落在入帳前（today<=attemptDay），維持待確認",
                    "trade_no", transaction.tradeNo());
            return ReconcileOutcome.INCONCLUSIVE;
        }

        // 30 天查詢窗口：嘗試日早於可查下限 → 無法完整涵蓋真實區間 → 無法斷定未撥 → INCONCLUSIVE（不盲目重發）。
        LocalDate floor = today.minusDays(GRANT_QUERY_MAX_LOOKBACK_DAYS);
        if (attemptDay.isBefore(floor)) {
            log.warn("PaymentApplicationService.grant.reconcile.outOfWindow",
                    "對帳起點超出 grant_query 30 天窗口，無法完整涵蓋，維持待確認",
                    "trade_no", transaction.tradeNo());
            return ReconcileOutcome.INCONCLUSIVE;
        }

        // 逐日 [attemptDay, today]，各日遍歷全分頁
        for (LocalDate d = attemptDay; !d.isAfter(today); d = d.plusDays(1)) {
            String queryDate = d.format(GRANT_QUERY_DATE_FMT);
            int page = 1;
            int pageTotal = 1;
            do {
                PayUniApiResponse resp;
                try {
                    resp = paymentClient.grantQuery(transaction.merId(), queryDate, page);
                } catch (PayUniApiException e) {
                    log.warn("PaymentApplicationService.grant.reconcile.queryException",
                            "grant_query 查詢例外，對帳未完成: " + e.getErrorCode() + " / " + e.getMessage(),
                            "trade_no", transaction.tradeNo());
                    return ReconcileOutcome.INCONCLUSIVE;   // 未查完 → 不重發
                }
                if (!GrantResponseRules.isGrantQueryDayComplete(resp.isSuccess(), resp.status())) {
                    // QUERY02004（超 30 天窗）/ 其他非預期 Status → 該日未查完 → 無法斷定，維持待確認、不盲目重發
                    log.warn("PaymentApplicationService.grant.reconcile.queryIncomplete",
                            "grant_query 該日未查完(" + resp.status() + ")，對帳無法斷定，維持待確認",
                            "trade_no", transaction.tradeNo());
                    return ReconcileOutcome.INCONCLUSIVE;   // 未查完 → 不重發
                }
                if (!resp.isSuccess()) {
                    // QUERY03002：該日查無符合撥款資料 = 有效空頁(count=0)。撥款隔日入帳，嘗試日當天恆空為常態，
                    // 不得誤判 INCONCLUSIVE（issue #1329）；跳出本日分頁，續掃下一日直到入帳日或全區間查完。
                    log.info("PaymentApplicationService.grant.reconcile.emptyDay",
                            "grant_query 該日查無撥款資料(" + resp.status() + ")，有效空頁，續掃下一日: date=" + queryDate,
                            "trade_no", transaction.tradeNo());
                    break;   // 有效空日 → 續下一日
                }
                int count = resp.getGrantQueryResultCount();
                for (int i = 0; i < count; i++) {
                    if (GrantResponseRules.isReconciliationHit(resp.getGrantQueryTradeNo(i), providerTradeNo)) {
                        log.info("PaymentApplicationService.grant.reconcile.matched",
                                "grant_query 命中已入帳: date=" + queryDate
                                        + ", type=" + resp.getGrantQueryType(i)
                                        + ", fundsAmt=" + resp.getGrantQueryFundsAmt(i)
                                        + ", executeAmt=" + resp.getGrantQueryExecuteAmt(i)
                                        + ", txAmount=" + transaction.amount(),
                                "trade_no", transaction.tradeNo());
                        return ReconcileOutcome.HIT_ALREADY_GRANTED;
                    }
                }
                pageTotal = resp.getGrantQueryPageTotal();
                page++;
            } while (page <= pageTotal && page <= GRANT_QUERY_MAX_PAGES_PER_DAY);
        }

        // 全區間 + 全分頁查完仍查無 → 確認未撥
        return ReconcileOutcome.CONFIRMED_NOT_GRANTED;
    }

    /**
     * 撥款指示 audit log（寫入 payment_logs 供對帳與爭議排查）
     */
    private void insertGrantLog(String tradeNo, String logLevel, String logType, String message) {
        PaymentLogEntity logEntity = new PaymentLogEntity();
        logEntity.setTradeNo(tradeNo);
        logEntity.setLogLevel(logLevel);
        logEntity.setLogType(logType);
        logEntity.setMessage(message);
        logEntity.setTriggerSource(TRIGGER_SOURCE_GRANT_API);
        paymentLogMapper.insert(logEntity);
    }

    // ==================== 內部方法 ====================

    /**
     * 處理信用卡支付（UPP 整合式支付頁）
     *
     * 根據 PAYUNi 官方文件 Ver 2.0，UPP 整合式支付頁要求使用 Form POST 方式：
     * - 返回 UppFormData 給前端
     * - 前端收到後建立 hidden form 並 POST 到 PAYUNi
     */
    private CreatePaymentResponse handleCreditCardPayment(PaymentTransaction transaction) {
        UppFormData uppFormData = paymentClient.createCreditCardPayment(
                transaction.merId(),
                transaction.tradeNo(),
                transaction.amount(),
                transaction.productName(),
                transaction.buyerId()
        );

        return CreatePaymentResponse.forCreditCard(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.amount(),
                uppFormData
        );
    }

    /**
     * 處理 Apple Pay 支付（依 §2A.8 ClientType 推導 mode）
     */
    private CreatePaymentResponse handleApplePayPayment(PaymentTransaction transaction, String applePayToken, String userEmail) {
        InputValidator.requireNotEmpty(applePayToken, "Apple Pay 付款憑證");

        PayUniApiResponse response = paymentClient.createApplePayPayment(
                transaction.merId(),
                transaction.tradeNo(),
                transaction.amount(),
                transaction.productName(),
                userEmail,
                applePayToken,
                resolveProviderMode(transaction.clientType())
        );

        // 解析回應並更新交易狀態
        PaymentTransactionStatus status = processLocalDateTimePaymentResponse(transaction, response);

        // 取得錯誤訊息（失敗時傳遞給前端）
        String errorMessage = (status == PaymentTransactionStatus.FAILED) ? response.getErrorMessage() : null;

        return CreatePaymentResponse.forApplePay(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.amount(),
                status,
                errorMessage
        );
    }

    /**
     * 處理 Google Pay 支付（依 §2A.8 ClientType 推導 mode）
     */
    private CreatePaymentResponse handleGooglePayPayment(PaymentTransaction transaction, String googlePayToken, String userEmail) {
        InputValidator.requireNotEmpty(googlePayToken, "Google Pay 付款憑證");

        PayUniApiResponse response = paymentClient.createGooglePayPayment(
                transaction.merId(),
                transaction.tradeNo(),
                transaction.amount(),
                transaction.productName(),
                userEmail,
                googlePayToken,
                resolveProviderMode(transaction.clientType())
        );

        // 解析回應並更新交易狀態
        PaymentTransactionStatus status = processLocalDateTimePaymentResponse(transaction, response);

        // 取得錯誤訊息（失敗時傳遞給前端）
        String errorMessage = (status == PaymentTransactionStatus.FAILED) ? response.getErrorMessage() : null;

        return CreatePaymentResponse.forGooglePay(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.amount(),
                status,
                errorMessage
        );
    }

    /**
     * 處理 ATM 支付
     *
     * @param transaction 交易記錄
     * @param expireDays  有效天數
     * @param bankType    銀行代碼（如 "812" 台新銀行）
     * @param userEmail   使用者信箱（選填，用於付款通知）
     */
    private CreatePaymentResponse handleAtmPayment(PaymentTransaction transaction, int expireDays, String bankType, String userEmail) {
        PayUniApiResponse response = paymentClient.createAtmPayment(
                transaction.merId(),
                transaction.tradeNo(),
                transaction.amount(),
                transaction.productName(),
                userEmail,
                expireDays,
                bankType
        );

        if (!response.isSuccess()) {
            log.error("PaymentApplicationService.atm.failed", "ATM 建立失敗: " + response.getErrorMessage(), "trade_no", transaction.tradeNo());
            throw new PaymentBusinessException(GrpcResponseCode.ATM_CREATION_FAILED, response.getErrorMessage());
        }

        // 更新 ATM 資訊
        // §10.4 D2/D3 修正：以 PayUni 回的 ExpireDate（格式 "yyyy-MM-dd HH:mm:ss" UTC+8）為唯一真實來源
        // 不再自己 nowUtc().plusSeconds(days * 86400) 計算
        String bankCode = response.getAtmBankCode();
        String virtualAccount = response.getAtmVirtualAccount();
        String expireDateStr = response.getAtmExpireDate();
        LocalDateTime expireAt = DateTimeUtils.parsePayUniDateTimeToUtc(expireDateStr);
        if (expireAt == null) {
            // 兜底：PayUni 回了「yyyy-MM-dd」純日期 → 視為當日 23:59:59 UTC+8
            expireAt = DateTimeUtils.payUniDateEndOfDayToUtc(expireDateStr);
        }
        LocalDate expireDate = expireAt != null ? expireAt.toLocalDate() : null;

        // §2A.3.6: 必須寫入 provider_trade_no（PayUni /api/atm 同步回應的 TradeNo），
        // 否則 §2A.6 webhook 段 1 findByProviderTradeNo 找不到，破壞三段查找
        transactionRepository.updateAtmInfo(
                transaction.tradeNo(),
                response.getProviderTradeNo(),
                bankCode,
                virtualAccount,
                expireDate,
                expireAt,
                transaction.version()
        );

        return CreatePaymentResponse.forAtm(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.amount(),
                bankCode,
                virtualAccount,
                expireDate,
                expireAt
        );
    }

    /**
     * 更新交易狀態
     */
    private void updateTransactionStatus(PaymentTransaction transaction, PaymentTransactionStatus newStatus, PayUniApiResponse response) {
        String tradeNo = transaction.tradeNo();
        int version = transaction.version();

        switch (newStatus) {
            case PAID -> {
                int updated = transactionRepository.updateToPaid(
                        tradeNo,
                        response.getProviderTradeNo(),
                        response.getTradeStatus(),
                        response.getErrorMessage(),
                        DateTimeUtils.nowUtc(),
                        version
                );
                if (updated > 0) {
                    PaymentDomainEvent.PaymentSucceeded event = domainService.createPaymentSucceededEvent(transaction, DateTimeUtils.nowUtc());
                    publishEvent(event);
                    log.info("PaymentApplicationService.status.paid", "交易付款成功", "trade_no", tradeNo);
                }
            }
            case FAILED -> {
                int updated = transactionRepository.updateToFailed(
                        tradeNo,
                        response.getTradeStatus(),
                        response.getErrorMessage(),
                        response.getProviderTradeNo(),
                        version
                );
                if (updated > 0) {
                    PaymentDomainEvent.PaymentFailed event = domainService.createPaymentFailedEvent(transaction, response.getErrorMessage());
                    publishEvent(event);
                    log.info("PaymentApplicationService.status.failed", "交易付款失敗", "trade_no", tradeNo);
                }
            }
            default -> log.info("PaymentApplicationService.status.unchanged", "交易狀態未變更", "trade_no", tradeNo);
        }
    }

    /**
     * §5.10.1 收斂點 B：付款結果事件（#2 零元 / #3 poll·reconcile / #4 Apple·Google Pay 同步）
     * 統一經 PaymentOutboxWriter 在當前 @Transactional 內寫入 outbox 並註冊 afterCommit 即時投遞，
     * 與交易狀態更新原子（rollback 皆不落地、commit 一併落地），消除事務內直發的 over-fulfillment 窗口。
     */
    private void publishEvent(PaymentDomainEvent event) {
        outboxWriter.enqueueAndDispatch(event);
    }

    /**
     * 處理幕後即時付回應（Apple Pay / Google Pay）
     *
     * @param transaction 交易記錄
     * @param response    PayUni API 回應
     * @return 交易狀態
     */
    private PaymentTransactionStatus processLocalDateTimePaymentResponse(PaymentTransaction transaction, PayUniApiResponse response) {
        String tradeNo = transaction.tradeNo();

        if (!response.isSuccess()) {
            log.error("PaymentApplicationService.instant.failed", "幕後即時付失敗: " + response.getErrorMessage(), "trade_no", tradeNo);

            // 更新為失敗狀態
            transactionRepository.updateToFailed(
                    tradeNo,
                    response.getTradeStatus(),
                    response.getErrorMessage(),
                    response.getProviderTradeNo(),
                    transaction.version()
            );

            // 發布失敗事件
            PaymentDomainEvent.PaymentFailed event = domainService.createPaymentFailedEvent(transaction, response.getErrorMessage());
            publishEvent(event);

            return PaymentTransactionStatus.FAILED;
        }

        // 解析 PayUni 回應狀態
        String providerStatus = response.getTradeStatus();
        Optional<PaymentTransactionStatus> statusOpt = domainService.parseProviderStatus(providerStatus);

        if (statusOpt.isPresent() && statusOpt.get() == PaymentTransactionStatus.PAID) {
            // 更新為已付款狀態
            int updated = transactionRepository.updateToPaid(
                    tradeNo,
                    response.getProviderTradeNo(),
                    providerStatus,
                    null,
                    DateTimeUtils.nowUtc(),
                    transaction.version()
            );

            if (updated > 0) {
                PaymentDomainEvent.PaymentSucceeded event = domainService.createPaymentSucceededEvent(transaction, DateTimeUtils.nowUtc());
                publishEvent(event);
                log.info("PaymentApplicationService.instant.success", "幕後即時付成功", "trade_no", tradeNo);
            }

            return PaymentTransactionStatus.PAID;
        }

        // 其他狀態視為失敗
        log.warn("PaymentApplicationService.instant.unknown", "幕後即時付未知狀態: " + providerStatus, "trade_no", tradeNo);
        return PaymentTransactionStatus.FAILED;
    }

    /**
     * 決定 ATM 有效天數：前端有傳就用前端值，否則用 application.yml 設定的預設值
     */
    private int resolveExpireDays(Integer requestExpireDays) {
        return requestExpireDays != null ? requestExpireDays : defaultExpireDays;
    }

    private String resolveClientType(String requested) {
        if (requested == null || requested.isBlank()) {
            return ClientType.MOBILE_APP.value();
        }
        if (!ClientType.isValidValue(requested)) {
            throw new PaymentBusinessException(GrpcResponseCode.INVALID_PARAM, "client_type=" + requested);
        }
        return requested;
    }

    /**
     * 依 ClientType 推導 PayUni Apple Pay / Google Pay 的 mode 參數（見 §2A.8）。
     * <p>
     * WEB_BROWSER → "1" (OnWeb)；MOBILE_APP / 空值 / 不可識別 → "2" (InApp)。
     * 空值或不可識別走 InApp 與既有 ClientType fallback 一致。
     */
    private String resolveProviderMode(String clientTypeValue) {
        if (clientTypeValue != null && ClientType.isValidValue(clientTypeValue)
                && ClientType.fromValue(clientTypeValue) == ClientType.WEB_BROWSER) {
            return PaymentClient.MODE_ON_WEB;
        }
        return PaymentClient.MODE_IN_APP;
    }

    private String resolveAppType(String requested) {
        if (requested == null || requested.isBlank()) {
            return AppType.BUYER.value();
        }
        if (!AppType.isValidValue(requested)) {
            throw new PaymentBusinessException(GrpcResponseCode.INVALID_PARAM, "app_type=" + requested);
        }
        return requested;
    }

    // §10.4 D2 修正：parseDate() 已刪除，改用 DateTimeUtils.parsePayUniDateTimeToUtc()
}
