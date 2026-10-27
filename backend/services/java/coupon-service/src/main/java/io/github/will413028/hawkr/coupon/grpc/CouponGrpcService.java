package io.github.will413028.hawkr.coupon.grpc;

import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.CouponInstanceStatus;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.CouponTargetType;
import io.github.will413028.hawkr.common.utils.code.CouponTemplateStatus;
import io.github.will413028.hawkr.common.utils.code.CouponType;
import io.github.will413028.hawkr.common.grpc.UnresolvedPrincipalException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.IssuerType;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.coupon.entity.CouponInstanceEntity;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.constant.CouponTargetTypeRules;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.coupon.grpc.*;
import io.github.will413028.hawkr.coupon.grpc.client.AccountGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.coupon.mapper.CouponTargetAccountMapper;
import io.github.will413028.hawkr.coupon.service.CouponCacheService;
import io.github.will413028.hawkr.coupon.service.CouponClaimService;
import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
import io.github.will413028.hawkr.coupon.service.CouponQuotaService;
import io.github.will413028.hawkr.coupon.service.CouponTemplateService;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;

@GrpcService
public class CouponGrpcService extends CouponServiceGrpc.CouponServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(CouponGrpcService.class);
    private final CouponTemplateService templateService;
    private final CouponInstanceService instanceService;
    private final CouponClaimService claimService;
    private final CouponQuotaService quotaService;
    private final CouponCacheService cacheService;
    private final SellerGrpcClient sellerGrpcClient;
    private final OrderGrpcClient orderGrpcClient;
    private final AccountGrpcClient accountGrpcClient;
    private final CouponTargetAccountMapper targetAccountMapper;
    private final BuyerGrpcClient buyerGrpcClient;

    public CouponGrpcService(CouponTemplateService templateService,
                              CouponInstanceService instanceService,
                              CouponClaimService claimService,
                              CouponQuotaService quotaService,
                              CouponCacheService cacheService,
                              SellerGrpcClient sellerGrpcClient,
                              OrderGrpcClient orderGrpcClient,
                              AccountGrpcClient accountGrpcClient,
                              CouponTargetAccountMapper targetAccountMapper,
                              BuyerGrpcClient buyerGrpcClient) {
        this.templateService = templateService;
        this.instanceService = instanceService;
        this.claimService = claimService;
        this.quotaService = quotaService;
        this.cacheService = cacheService;
        this.sellerGrpcClient = sellerGrpcClient;
        this.orderGrpcClient = orderGrpcClient;
        this.accountGrpcClient = accountGrpcClient;
        this.targetAccountMapper = targetAccountMapper;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    // ==================== Buyer ====================

    @Override
    public void listMyCoupons(ListMyCouponsRequest request, StreamObserver<ListMyCouponsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();
            long rawAccountId = request.getAccountId();

            int page = parseIntOrDefault(request.getPage(), 1);
            int pageSize = parseIntOrDefault(request.getPageSize(), 20);

            // §6.8.1 status 解析：只認 CouponInstanceStatus.value()，禁止 code 字面比對
            // - 不傳 / 空 → 兩者皆查
            // - "0" → 僅 UNUSED；"2" → 僅 USED；"0,2" → 兩者
            // - 其他值 → INVALID_PARAM（禁止靜默回空）
            String statusParam = request.getStatus();
            boolean wantUnused;
            boolean wantUsed;
            if (statusParam == null || statusParam.isBlank()) {
                wantUnused = true;
                wantUsed = true;
            } else {
                wantUnused = false;
                wantUsed = false;
                for (String raw : statusParam.split(",")) {
                    String v = raw.trim();
                    if (v.isEmpty()) continue;
                    CodeMetaValidator.requireValidValue(CouponInstanceStatus.class, v, "status");
                    CouponInstanceStatus s = CouponInstanceStatus.fromValue(v);
                    if (s == CouponInstanceStatus.UNUSED) wantUnused = true;
                    else if (s == CouponInstanceStatus.USED) wantUsed = true;
                    // 其他合法 value（RESERVED/EXPIRED/REVOKED/ANNUL）目前不對應任何分支，安全忽略
                }
            }

            var builder = ListMyCouponsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            // ========== 可用券（records）：可見性規則下放至 SQL，過濾先於分頁（§6.7） ==========
            if (wantUnused) {
                // 預先載入跨服務資料（request scope 快取）
                long buyerCreatedAt = accountGrpcClient.getAccountCreatedAt(rawAccountId);
                Set<Long> memberSellerIds = orderGrpcClient.getBuyerMemberSellerIds(buyerId);

                // §6.8：在 SQL 層套用 4 種 targetType 對稱可見性條件
                Page<CouponTemplateEntity> templateResult = templateService.listAvailableForBuyer(
                        page, pageSize, null, buyerId, buyerCreatedAt, memberSellerIds);

                // 查詢已有 USED/RESERVED Instance 的 template_id（排除已用）
                Set<Long> usedTemplateIds = instanceService.listByOwner(buyerId, null, 1, 9999)
                        .getRecords().stream()
                        .filter(inst -> CouponInstanceStatus.USED.value().equals(inst.getStatus())
                                || CouponInstanceStatus.RESERVED.value().equals(inst.getStatus()))
                        .map(CouponInstanceEntity::getTemplateId)
                        .collect(java.util.stream.Collectors.toSet());

                // 批次查詢已領取的 template_id
                Set<Long> claimedTemplateIds = claimService.getClaimedTemplateIds(buyerId);

                List<CouponTemplateEntity> eligible = new ArrayList<>();
                for (CouponTemplateEntity tpl : templateResult.getRecords()) {
                    if (usedTemplateIds.contains(tpl.getTemplateId())) continue;
                    eligible.add(tpl);
                }

                Map<Long, SellerMeta> sellerMetaMap = batchGetSellerMeta(eligible.stream()
                        .map(CouponTemplateEntity::getApplicableStoreId)
                        .filter(Objects::nonNull).distinct().collect(Collectors.toList()));

                for (CouponTemplateEntity tpl : eligible) {
                    builder.addRecords(toTemplateInfoWithClaim(tpl,
                            sellerMetaOf(sellerMetaMap, tpl.getApplicableStoreId()),
                            Boolean.TRUE.equals(tpl.getClaimRequired()),
                            claimedTemplateIds.contains(tpl.getTemplateId())));
                }
                builder.setTotal(String.valueOf(eligible.size()));
            }

            // ========== 已使用券（used_records）==========
            if (wantUsed) {
                Page<CouponInstanceEntity> usedResult = instanceService.listByOwner(
                        buyerId, CouponInstanceStatus.USED.value(), page, pageSize);

                Map<Long, SellerMeta> usedSellerMetaMap = batchGetSellerMeta(usedResult.getRecords().stream()
                        .map(CouponInstanceEntity::getApplicableStoreId)
                        .filter(Objects::nonNull).distinct().collect(Collectors.toList()));

                for (CouponInstanceEntity inst : usedResult.getRecords()) {
                    CouponTemplateEntity tpl = getTemplateWithCache(inst.getTemplateId());
                    builder.addUsedRecords(toInstanceInfo(inst, tpl,
                            sellerMetaOf(usedSellerMetaMap, inst.getApplicableStoreId())));
                }
            }

            builder.setPage(String.valueOf(page)).setPageSize(String.valueOf(pageSize));
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ListMyCouponsResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ListMyCouponsResponse.newBuilder(), e, "listMyCoupons");
        }
    }

    @Override
    public void listAvailableCoupons(ListAvailableCouponsRequest request,
                                      StreamObserver<ListAvailableCouponsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();
            long rawAccountId = request.getAccountId();

            int page = parseIntOrDefault(request.getPage(), 1);
            int pageSize = parseIntOrDefault(request.getPageSize(), 20);
            Long storeId = parseNullableLong(request.getStoreId());
            Long orderAmount = parseNullableLong(request.getOrderAmount());

            // 跨服務資料
            long buyerCreatedAt = accountGrpcClient.getAccountCreatedAt(rawAccountId);
            Set<Long> memberSellerIds = orderGrpcClient.getBuyerMemberSellerIds(buyerId);

            // §6.9：在 SQL 層套用 4 種 targetType 對稱可見性條件，過濾先於分頁
            Page<CouponTemplateEntity> result = templateService.listAvailableForBuyer(
                    page, pageSize, storeId, buyerId, buyerCreatedAt, memberSellerIds);

            // 已用/鎖定的 template_id
            Set<Long> usedTemplateIds = instanceService.listByOwner(buyerId, null, 1, 9999)
                    .getRecords().stream()
                    .filter(inst -> CouponInstanceStatus.USED.value().equals(inst.getStatus())
                            || CouponInstanceStatus.RESERVED.value().equals(inst.getStatus()))
                    .map(CouponInstanceEntity::getTemplateId)
                    .collect(java.util.stream.Collectors.toSet());

            // 已領取的 template_id（用於過濾 claim_required 但未領取的券）
            Set<Long> claimedTemplateIds = claimService.getClaimedTemplateIds(buyerId);

            List<CouponTemplateEntity> eligible = new ArrayList<>();
            for (CouponTemplateEntity tpl : result.getRecords()) {
                if (usedTemplateIds.contains(tpl.getTemplateId())) continue;
                // 門檻篩選
                if (orderAmount != null && tpl.getThresholdAmount() != null && orderAmount < tpl.getThresholdAmount()) continue;
                // 結帳頁排除「需領取但未領」的券（與可見性正交的維度）
                if (Boolean.TRUE.equals(tpl.getClaimRequired())
                        && !claimedTemplateIds.contains(tpl.getTemplateId())) {
                    continue;
                }
                eligible.add(tpl);
            }

            Map<Long, SellerMeta> sellerMetaMap = batchGetSellerMeta(eligible.stream()
                    .map(CouponTemplateEntity::getApplicableStoreId)
                    .filter(Objects::nonNull).distinct().collect(Collectors.toList()));

            var builder = ListAvailableCouponsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setTotal(String.valueOf(eligible.size()))
                    .setPage(String.valueOf(page))
                    .setPageSize(String.valueOf(pageSize));

            for (CouponTemplateEntity tpl : eligible) {
                SellerMeta tplSellerMeta = sellerMetaOf(sellerMetaMap, tpl.getApplicableStoreId());
                // 計算預算折扣
                long discount = 0;
                if (orderAmount != null) {
                    discount = instanceService.calculateDiscountForTemplate(tpl, orderAmount);
                }
                builder.addRecords(AvailableCouponInfo.newBuilder()
                        .setTemplate(toTemplateInfo(tpl, tplSellerMeta))
                        .setEstimatedDiscount(String.valueOf(discount))
                        .build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ListAvailableCouponsResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ListAvailableCouponsResponse.newBuilder(), e, "listAvailableCoupons");
        }
    }

    @Override
    public void buyerClaimCoupon(BuyerClaimCouponRequest request,
                                  StreamObserver<BuyerClaimCouponResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();
            long rawAccountId = request.getAccountId();
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            // 1. 取得模板，驗證存在
            CouponTemplateEntity template = templateService.getActiveTemplate(templateId);

            // 2. 驗證 ACTIVE 狀態 + 時間有效
            if (!CouponTemplateStatus.ACTIVE.value().equals(template.getStatus())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_CLAIM_REQUIRES_ACTIVE);
            }
            LocalDateTime now = DateTimeUtils.nowUtc();
            if (now.isBefore(template.getStartAt()) || now.isAfter(template.getEndAt())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_CLAIM_OUT_OF_WINDOW);
            }

            // 3. 驗證 claim_required = TRUE
            if (!Boolean.TRUE.equals(template.getClaimRequired())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_CLAIM_NOT_REQUIRED,
                        GrpcResponseCode.COUPON_CLAIM_NOT_REQUIRED.message());
            }

            // 4. target_type 資格檢查（與 listMyCoupons 相同邏輯）
            long buyerCreatedAt = accountGrpcClient.getAccountCreatedAt(rawAccountId);
            Set<Long> memberSellerIds = orderGrpcClient.getBuyerMemberSellerIds(buyerId);
            List<Long> designatedTemplateIds = targetAccountMapper.selectTemplateIdsByBuyerId(buyerId);
            if (!isEligible(template, buyerId, buyerCreatedAt, memberSellerIds, designatedTemplateIds)) {
                throw new CouponBusinessException(GrpcResponseCode.PERMISSION_DENIED,
                        "不符合此優惠券的領取資格");
            }

            // 5. total_issue_limit 檢查
            if (template.getTotalIssueLimit() != -1
                    && template.getIssuedCount() >= template.getTotalIssueLimit()) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_ISSUE_LIMIT_REACHED,
                        GrpcResponseCode.COUPON_ISSUE_LIMIT_REACHED.message());
            }

            // 6. 領取（冪等）
            LocalDateTime claimedAt = claimService.claimCoupon(templateId, buyerId);

            // 7. 回傳
            BuyerClaimCouponResponse response = BuyerClaimCouponResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setSuccess(true)
                    .setClaimedAt(formatDateTime(claimedAt))
                    .build();
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, BuyerClaimCouponResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, BuyerClaimCouponResponse.newBuilder(), e, "buyerClaimCoupon");
        }
    }

    // ==================== Order Internal ====================

    @Override
    public void validateCoupon(ValidateCouponRequest request, StreamObserver<ValidateCouponResponse> responseObserver) {
        try {
            Long templateId = parseLong(request.getTemplateId(), "template_id");
            long buyerId = request.getBuyerId();
            Long storeId = parseNullableLong(request.getStoreId());
            Long orderAmount = parseLong(request.getOrderAmount(), "order_amount");

            CouponInstanceService.ValidateResult result = instanceService.validateCoupon(
                    templateId, buyerId, storeId, orderAmount);

            responseObserver.onNext(ValidateCouponResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(CouponValidateResult.newBuilder()
                            .setValid(result.valid())
                            .setDiscountAmount(String.valueOf(result.discountAmount()))
                            .setDiscountType(result.discountType())
                            .setDescription(result.description())
                            .setTemplateId(String.valueOf(templateId))
                            .build())
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ValidateCouponResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ValidateCouponResponse.newBuilder(), e, "validateCoupon");
        }
    }

    @Override
    public void reserveCoupon(ReserveCouponRequest request, StreamObserver<ReserveCouponResponse> responseObserver) {
        try {
            Long templateId = parseLong(request.getTemplateId(), "template_id");
            long buyerId = request.getBuyerId();
            String orderNo = request.getOrderNo();

            Long instanceId = instanceService.reserveCoupon(templateId, buyerId, orderNo, null);

            responseObserver.onNext(ReserveCouponResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setSuccess(true)
                    .setInstanceId(String.valueOf(instanceId))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ReserveCouponResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ReserveCouponResponse.newBuilder(), e, "reserveCoupon");
        }
    }

    @Override
    public void consumeCoupon(ConsumeCouponRequest request, StreamObserver<ConsumeCouponResponse> responseObserver) {
        try {
            Long instanceId = parseLong(request.getInstanceId(), "instance_id");
            String orderNo = request.getOrderNo();

            boolean success = instanceService.consumeCoupon(instanceId, orderNo);

            responseObserver.onNext(ConsumeCouponResponse.newBuilder()
                    .setCode(success ? GrpcResponseCode.SUCCESS.code() : GrpcResponseCode.COUPON_INVALID_STATUS.code())
                    .setMessage(success ? "success" : "消耗失敗")
                    .setSuccess(success)
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ConsumeCouponResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ConsumeCouponResponse.newBuilder(), e, "consumeCoupon");
        }
    }

    @Override
    public void releaseCoupon(ReleaseCouponRequest request, StreamObserver<ReleaseCouponResponse> responseObserver) {
        try {
            Long instanceId = parseLong(request.getInstanceId(), "instance_id");
            String orderNo = request.getOrderNo();

            boolean success = instanceService.releaseCoupon(instanceId, orderNo);

            responseObserver.onNext(ReleaseCouponResponse.newBuilder()
                    .setCode(success ? GrpcResponseCode.SUCCESS.code() : GrpcResponseCode.COUPON_INVALID_STATUS.code())
                    .setMessage(success ? "success" : "釋放失敗")
                    .setSuccess(success)
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ReleaseCouponResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ReleaseCouponResponse.newBuilder(), e, "releaseCoupon");
        }
    }

    // ==================== Seller ====================

    /**
     * 自發自用防護（resolve-to-principal，Plan 2b）：賣家側 id 解析出的 principal
     * 若跟買家側任一 id 解析出的 principal 相同即拋 COUPON_SELF_ISSUE_NOT_ALLOWED。
     * package-visible 供測試直接呼叫，不需要組出完整 gRPC request/StreamObserver。
     *
     * <p>{@code buyerIds} 是 DESIGNATED 模式的 {@code buyer_ids} 欄位，值是 gateway
     * 已解析完成的 buyer_id，不是登入 account_id（2026-08-05 命名清掃前這個參數叫
     * {@code buyerAccountIds}，名字本身就是這條 bug 的溫床）。修復前直接拿
     * sellerPrincipal（account_id）跟這個 buyer_id 列表比對，兩者不是同一個 id 空間：
     * 分岔賣家（自己的 buyer_id ≠ account_id）可以對「自己現在的 buyer_id」發 DESIGNATED
     * 優惠券，繞過本防護（account_id/store_id 稽核 2026-08-01 Bug 4）。修法對齊
     * cart-service CartService#addItem 的 resolveSellerPrincipals/resolveBuyerPrincipal
     * 寫法：兩側都先 resolve 成 account_id 才比較。
     */
    static void enforceNoSelfIssueForTest(SellerGrpcClient sellerGrpcClient, BuyerGrpcClient buyerGrpcClient,
                                           long storeId, List<Long> buyerIds) {
        // fail-closed（issue #2789 階段 B1）：兩側都必須是<b>真正解析出來</b>的 account_id。
        // seller/buyer service 對查無在架列的 id 會把輸入原樣回吐成合法非零值，舊寫法
        // getOrDefault(id, id) 分辨不出來 ⇒「無法確認是不是同一個人」被當成「不是同一個人」
        // ⇒ 自發自用防護靜默失效。未解析一律拒絕，不拿 echo-back 做身分比對。
        long sellerPrincipal;
        Map<Long, Long> buyerPrincipals;
        try {
            sellerPrincipal = sellerGrpcClient.resolvePrincipals(List.of(storeId))
                    .requireResolved(storeId, "store_id");
        } catch (UnresolvedPrincipalException e) {
            log.warn("CouponGrpcService.enforceNoSelfIssue.storeUnresolved",
                    "store_id 查無在架賣家，自發自用防護無法評估，fail-closed 拒絕發券：" + e.getMessage(),
                    "storeId", String.valueOf(storeId));
            throw new CouponBusinessException(GrpcResponseCode.SELLER_NOT_FOUND,
                    GrpcResponseCode.SELLER_NOT_FOUND.message());
        }
        try {
            buyerPrincipals = buyerGrpcClient.resolvePrincipals(buyerIds)
                    .requireAllResolved(buyerIds, "buyer_id");
        } catch (UnresolvedPrincipalException e) {
            log.warn("CouponGrpcService.enforceNoSelfIssue.buyerUnresolved",
                    "指定的 buyer_id 查無在架買家，自發自用防護無法評估，fail-closed 拒絕發券："
                            + e.getMessage(),
                    "storeId", String.valueOf(storeId));
            // 防枚舉（API 設計規範 §14.1）：不把「哪些 buyer_id 不存在」回給呼叫端，
            // 否則這個錯誤就成了買家存在性的探測器。具體 id 只進 log。
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "buyer_ids 含無法驗證的買家");
        }
        boolean targetsSelf = buyerIds.stream()
                .anyMatch(buyerId -> buyerPrincipals.get(buyerId) == sellerPrincipal);
        if (targetsSelf) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_SELF_ISSUE_NOT_ALLOWED,
                    GrpcResponseCode.COUPON_SELF_ISSUE_NOT_ALLOWED.message());
        }
    }

    @Override
    public void sellerCreateTemplate(SellerCreateTemplateRequest request,
                                      StreamObserver<SellerCreateTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            // 配額檢查
            quotaService.checkQuota(storeId);

            // 驗證 target_type
            String targetType = request.getTargetType();
            CodeMetaValidator.requireValidValue(CouponTargetType.class, targetType, "target_type");
            CouponTargetType targetTypeEnum = CouponTargetType.fromValue(targetType);
            if (!CouponTargetTypeRules.isSellerAllowed(targetTypeEnum)) {
                throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "賣家不允許的 target_type: " + targetType);
            }

            // DESIGNATED 時必須提供買家列表
            List<Long> buyerIds = new ArrayList<>();
            if (CouponTargetType.DESIGNATED.value().equals(targetType)) {
                for (String idStr : request.getBuyerIdsList()) {
                    buyerIds.add(parseLong(idStr, "buyer_ids"));
                }
                if (buyerIds.isEmpty()) {
                    throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                            "DESIGNATED 模式下 buyer_ids 不可為空");
                }
                // 自發自用防護（resolve-to-principal，Plan 2b）。RPC 失敗會拋
                // StatusRuntimeException，直接交給本方法外層既有的 catch (Exception e)
                // → onSystemError 處理（fail-closed：模板不會建立）。
                enforceNoSelfIssueForTest(sellerGrpcClient, buyerGrpcClient, storeId, buyerIds);
            }

            // 建立模板
            CouponTemplateEntity template = templateService.createSellerTemplate(
                    storeId,
                    request.getTitle(),
                    request.getType(),
                    targetType,
                    parseNullableLong(request.getThresholdAmount()),
                    parseNullableLong(request.getDiscountAmount()),
                    parseNullableInt(request.getDiscountPercent()),
                    parseNullableLong(request.getMaxDiscountAmount()),
                    parseDateTimeRequired(request.getStartAt(), "start_at"),
                    parseDateTimeRequired(request.getEndAt(), "end_at"));

            // DESIGNATED 時寫入 coupon_target_accounts
            if (CouponTargetType.DESIGNATED.value().equals(targetType)) {
                insertTargetAccounts(template.getTemplateId(), buyerIds);
            }

            cacheService.evictAvailableList();

            responseObserver.onNext(SellerCreateTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toTemplateInfo(template))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(SellerCreateTemplateResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, SellerCreateTemplateResponse.newBuilder(), e, "sellerCreateTemplate");
        }
    }

    @Override
    public void sellerListTemplates(SellerListTemplatesRequest request,
                                     StreamObserver<SellerListTemplatesResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            int page = parseIntOrDefault(request.getPage(), 1);
            int pageSize = parseIntOrDefault(request.getPageSize(), 20);

            Page<CouponTemplateEntity> result = templateService.listSellerTemplates(
                    storeId, request.getStatus(), page, pageSize);

            int maxCount = quotaService.getMaxTemplateCount(storeId);
            long activeCount = templateService.countSellerActiveTemplates(storeId);

            var builder = SellerListTemplatesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()))
                    .setMaxTemplateCount(String.valueOf(maxCount))
                    .setCurrentActiveCount(String.valueOf(activeCount));

            for (CouponTemplateEntity tpl : result.getRecords()) {
                builder.addRecords(toTemplateInfo(tpl));
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, SellerListTemplatesResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, SellerListTemplatesResponse.newBuilder(), e, "sellerListTemplates");
        }
    }

    @Override
    public void sellerGetTemplateDetail(SellerGetTemplateDetailRequest request,
                                         StreamObserver<SellerGetTemplateDetailResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            CouponTemplateEntity template = templateService.getActiveTemplate(templateId);
            // issuer_store_id 現為 SELLER-only 欄位（V6 migration 的寫入路徑保證：ADMIN 建立
            // 路徑寫 issuer_account_id、SELLER 建立路徑寫 issuer_store_id，見
            // CouponTemplateService）。但 V6 這次只做 expand（見該 migration 檔頭註解），DB 層
            // 的 chk_tpl_issuer_identity CHECK constraint 依 expand-contract 規範延後到 contract
            // release 才加，尚未真的存在 —— 這段 issuer_type=SELLER 判斷目前是唯一擋線，不是
            // DB CHECK 之外的第二層防線。先確認 issuer_type=SELLER 才比對 issuer_store_id：一是
            // 避免對 null 呼叫 .equals() 產生 NPE，二是維持越權防枚舉語意——非本店家（含 ADMIN
            // 發的）模板一律當「不存在」。
            if (!IssuerType.SELLER.value().equals(template.getIssuerType())
                    || !template.getIssuerStoreId().equals(storeId)) {
                // 越權防枚舉（API 設計規範 §14.1）：回應與「模板不存在」一致
                throw new CouponBusinessException(GrpcResponseCode.COUPON_NOT_FOUND);
            }

            int usedCount = instanceService.countByTemplateAndStatus(templateId, CouponInstanceStatus.USED.value());
            int reservedCount = instanceService.countByTemplateAndStatus(templateId, CouponInstanceStatus.RESERVED.value());

            // 建立基本 TemplateInfo
            TemplateInfo.Builder templateInfoBuilder = toTemplateInfo(template).toBuilder();

            // DESIGNATED / COMPENSATION 時查詢指定帳號並聚合暱稱 + 電話
            String targetType = template.getTargetType();
            if (CouponTargetType.DESIGNATED.value().equals(targetType)
                    || CouponTargetType.COMPENSATION.value().equals(targetType)) {
                List<Long> buyerIds = targetAccountMapper.selectBuyerIdsByTemplateId(templateId);
                if (!buyerIds.isEmpty()) {
                    // buyer_id 是 stint-scoped 身分，AccountGrpcClient 是用 raw account_id
                    // 當主鍵查 accounts 表 —— 分岔買家（buyer_id != 目前登入 account_id）
                    // 若直接把 buyer_id 丟進去查會找不到人，nickname/phone 靜默回空字串
                    // （buyer_id rekey Phase 5 既有缺口，見 coupon.proto TargetAccountInfo
                    // 註解）。resolvePrincipals 失敗（RPC 例外）或某 buyer_id 沒有解析到存活的買家時，那位收券人
                    // 不查帳號資訊（沒有電話），不拿 buyer_id 冒充 account_id 去查別人的電話；不阻斷本次查詢。
                    PrincipalResolution resolvedPrincipals;
                    try {
                        resolvedPrincipals = buyerGrpcClient.resolvePrincipals(buyerIds);
                    } catch (Exception e) {
                        log.error("CouponGrpcService.sellerGetTemplateDetail.resolvePrincipalsFailed",
                                "buyer_id -> account_id 批次反查失敗，收券人的電話欄位留空", e);
                        resolvedPrincipals = PrincipalResolution.empty();
                    }
                    // 每位收券人只解析一次（未解析的只記一行 warn）；0 表示沒有解析到存活的買家。
                    PrincipalResolution principalsByBuyerId = resolvedPrincipals;
                    Map<Long, Long> accountIdByBuyerId = new java.util.LinkedHashMap<>();
                    for (Long buyerId : buyerIds) {
                        accountIdByBuyerId.computeIfAbsent(buyerId,
                                id -> principalsByBuyerId.principalOrZero(id, "buyer_id"));
                    }
                    List<Long> resolvedAccountIds = accountIdByBuyerId.values().stream()
                            .filter(accountId -> accountId > 0L)
                            .collect(Collectors.toList());
                    var accountInfoMap = resolvedAccountIds.isEmpty()
                            ? Map.<Long, io.github.will413028.hawkr.account.grpc.AccountInfo>of()
                            : accountGrpcClient.batchGetAccountInfo(resolvedAccountIds);
                    // 名字只有 buyer-service 有：accounts 表沒有 nickname 欄位，
                    // AccountInfo.nickname 對每一位收券人都是空字串（不只已註銷者）。
                    // 走 Display 版讓已註銷的收券人也顯示得出來（回墓碑字串）。#2830
                    Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> buyerDisplayMap;
                    try {
                        buyerDisplayMap = buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(buyerIds);
                    } catch (Exception e) {
                        log.error("CouponGrpcService.sellerGetTemplateDetail.buyerDisplayFailed",
                                "收券人顯示名批次查詢失敗，清單將只有電話", e);
                        buyerDisplayMap = Map.of();
                    }
                    for (Long buyerId : buyerIds) {
                        long resolvedAccountId = accountIdByBuyerId.get(buyerId);
                        var builder = TargetAccountInfo.newBuilder()
                                .setBuyerId(String.valueOf(buyerId));
                        var display = buyerDisplayMap.get(buyerId);
                        if (display != null) {
                            builder.setNickname(display.getDisplayName());
                        }
                        var info = resolvedAccountId > 0L ? accountInfoMap.get(resolvedAccountId) : null;
                        if (info != null) {
                            builder.setPhone(info.getPhone());
                        }
                        templateInfoBuilder.addTargetAccounts(builder.build());
                    }
                }
            }

            responseObserver.onNext(SellerGetTemplateDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(templateInfoBuilder.build())
                    .setUsedCount(String.valueOf(usedCount))
                    .setReservedCount(String.valueOf(reservedCount))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(SellerGetTemplateDetailResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, SellerGetTemplateDetailResponse.newBuilder(), e, "sellerGetTemplateDetail");
        }
    }

    @Override
    public void sellerRevokeTemplate(SellerRevokeTemplateRequest request,
                                      StreamObserver<SellerRevokeTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            boolean revoked = templateService.revokeTemplate(templateId, storeId, request.getReason());
            if (!revoked) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_INVALID_STATUS);
            }

            int revokedCount = instanceService.revokeByTemplate(templateId, request.getReason());
            cacheService.evictTemplate(templateId);
            cacheService.evictAvailableList();

            responseObserver.onNext(SellerRevokeTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setRevokedInstanceCount(String.valueOf(revokedCount))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(SellerRevokeTemplateResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, SellerRevokeTemplateResponse.newBuilder(), e, "sellerRevokeTemplate");
        }
    }

    @Override
    public void listBuyerCouponsForSeller(ListBuyerCouponsForSellerRequest request,
                                           StreamObserver<ListBuyerCouponsForSellerResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            Long buyerId = parseLong(request.getBuyerId(), "buyer_id");

            int page = parseIntOrDefault(request.getPage(), 1);
            int pageSize = parseIntOrDefault(request.getPageSize(), 20);

            Page<CouponInstanceEntity> result = instanceService.listBuyerCouponsForSeller(
                    storeId, buyerId, request.getStatus(), page, pageSize);

            // 批量查詢賣家名稱
            Map<Long, SellerMeta> sellerMetaMap = batchGetSellerMeta(result.getRecords().stream()
                    .map(CouponInstanceEntity::getApplicableStoreId)
                    .filter(Objects::nonNull)
                    .distinct()
                    .collect(Collectors.toList()));

            var builder = ListBuyerCouponsForSellerResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()));

            for (CouponInstanceEntity inst : result.getRecords()) {
                CouponTemplateEntity tpl = getTemplateWithCache(inst.getTemplateId());
                builder.addUsedRecords(toInstanceInfo(inst, tpl,
                        sellerMetaOf(sellerMetaMap, inst.getApplicableStoreId())));
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, ListBuyerCouponsForSellerResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, ListBuyerCouponsForSellerResponse.newBuilder(), e, "listBuyerCouponsForSeller");
        }
    }

    // ==================== Admin ====================

    @Override
    public void adminCreateTemplate(AdminCreateTemplateRequest request,
                                     StreamObserver<AdminCreateTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminAccountId = GrpcAuthContext.requireAccountId();

            String targetType = request.getTargetType();
            CodeMetaValidator.requireValidValue(CouponTargetType.class, targetType, "target_type");
            CodeMetaValidator.requireValidValue(CouponType.class, request.getType(), "type");

            List<Long> buyerIds = new ArrayList<>();
            for (String id : request.getBuyerIdsList()) {
                buyerIds.add(Long.parseLong(id));
            }

            // claim_required: proto3 預設 false，但 createTemplate 預設 true
            // Gateway 應在 admin 未指定時傳 true
            Boolean claimRequired = request.getClaimRequired();

            CouponTemplateEntity template = templateService.createTemplate(
                    adminAccountId,
                    request.getTitle(),
                    request.getType(),
                    targetType,
                    parseNullableLong(request.getApplicableStoreId()),
                    parseNullableLong(request.getThresholdAmount()),
                    parseNullableLong(request.getDiscountAmount()),
                    parseNullableInt(request.getDiscountPercent()),
                    parseNullableLong(request.getMaxDiscountAmount()),
                    parseDateTimeRequired(request.getStartAt(), "start_at"),
                    parseDateTimeRequired(request.getEndAt(), "end_at"),
                    null,  // usageLimitPerAccount (default 1)
                    parseNullableInt(request.getTotalIssueLimit()),
                    buyerIds,
                    claimRequired);

            // DESIGNATED / COMPENSATION 時寫入 coupon_target_accounts
            if (CouponTargetTypeRules.requiresTargetAccounts(CouponTargetType.fromValue(targetType))
                    && !buyerIds.isEmpty()) {
                insertTargetAccounts(template.getTemplateId(), buyerIds);
            }

            responseObserver.onNext(AdminCreateTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toTemplateInfo(template))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminCreateTemplateResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminCreateTemplateResponse.newBuilder(), e, "adminCreateTemplate");
        }
    }

    @Override
    public void adminPublishTemplate(AdminPublishTemplateRequest request,
                                      StreamObserver<AdminPublishTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            templateService.publishTemplate(templateId);
            cacheService.evictAvailableList();

            responseObserver.onNext(AdminPublishTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message()).build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminPublishTemplateResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminPublishTemplateResponse.newBuilder(), e, "adminPublishTemplate");
        }
    }

    @Override
    public void adminDisableTemplate(AdminDisableTemplateRequest request,
                                      StreamObserver<AdminDisableTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            templateService.disableTemplate(templateId);
            cacheService.evictTemplate(templateId);
            cacheService.evictAvailableList();

            responseObserver.onNext(AdminDisableTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message()).build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminDisableTemplateResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminDisableTemplateResponse.newBuilder(), e, "adminDisableTemplate");
        }
    }

    @Override
    public void adminIssueCoupons(AdminIssueCouponsRequest request,
                                   StreamObserver<AdminIssueCouponsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminAccountId = GrpcAuthContext.requireAccountId();
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            CouponTemplateEntity template = templateService.getActiveTemplate(templateId);
            if (!CouponTemplateStatus.ACTIVE.value().equals(template.getStatus())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_INVALID_STATUS);
            }

            List<Long> buyerIds = new ArrayList<>();
            for (String idStr : request.getBuyerIdsList()) {
                buyerIds.add(parseLong(idStr, "buyer_ids"));
            }

            // 寫入 coupon_target_accounts
            insertTargetAccounts(templateId, buyerIds);
            cacheService.evictAvailableList();

            // immediate_distribute: 立即代領
            int distributedCount = 0;
            if (request.getImmediateDistribute() && !buyerIds.isEmpty()) {
                distributedCount = claimService.batchClaimForBuyers(templateId, buyerIds);
            }

            responseObserver.onNext(AdminIssueCouponsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setIssuedCount(String.valueOf(buyerIds.size()))
                    .setDistributedCount(String.valueOf(distributedCount))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, AdminIssueCouponsResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, AdminIssueCouponsResponse.newBuilder(), e, "adminIssueCoupons");
        }
    }

    @Override
    public void adminDistributeTemplate(AdminDistributeTemplateRequest request,
                                         StreamObserver<AdminDistributeTemplateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long templateId = parseLong(request.getTemplateId(), "template_id");

            // 驗證模板存在且 ACTIVE
            CouponTemplateEntity template = templateService.getActiveTemplate(templateId);
            if (!CouponTemplateStatus.ACTIVE.value().equals(template.getStatus())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_DISTRIBUTE_REQUIRES_ACTIVE);
            }

            // 驗證 claim_required = TRUE
            if (!Boolean.TRUE.equals(template.getClaimRequired())) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_CLAIM_NOT_REQUIRED,
                        GrpcResponseCode.COUPON_CLAIM_NOT_REQUIRED.message());
            }

            // 取得白名單帳號
            List<Long> targetAccountIds = targetAccountMapper.selectBuyerIdsByTemplateId(templateId);
            int totalTargetCount = targetAccountIds.size();

            // 批量代領（INSERT IGNORE 冪等）
            int distributedCount = 0;
            if (!targetAccountIds.isEmpty()) {
                distributedCount = claimService.batchClaimForBuyers(templateId, targetAccountIds);
            }
            int alreadyClaimedCount = totalTargetCount - distributedCount;

            responseObserver.onNext(AdminDistributeTemplateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setDistributedCount(String.valueOf(distributedCount))
                    .setAlreadyClaimedCount(String.valueOf(alreadyClaimedCount))
                    .setTotalTargetCount(String.valueOf(totalTargetCount))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            onBusinessError(responseObserver, AdminDistributeTemplateResponse.newBuilder(), e);
        } catch (Exception e) {
            onSystemError(responseObserver, AdminDistributeTemplateResponse.newBuilder(), e, "adminDistributeTemplate");
        }
    }

    @Override
    public void adminGrantCompensationCoupon(AdminGrantCompensationCouponRequest request,
                                              StreamObserver<AdminGrantCompensationCouponResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminAccountId = GrpcAuthContext.requireAccountId();
            Long buyerId = parseLong(request.getBuyerId(), "buyer_id");
            Long amount = parseLong(request.getAmount(), "amount");
            if (amount <= 0) {
                throw new IllegalArgumentException("補償金額 必須大於 0");
            }
            // 適用商店必填。這裡刻意不用 parseNullableLong——它把空字串「和」格式
            // 錯誤都吞成 null，而 null 在 Scope 檢查裡等同「全平台任何商店都能用」
            // （設計書 07 §2.3），等於讓一個打錯的欄位名把限店補償券放大成全平台
            // 券。缺值／格式錯一律 INVALID_PARAM。
            Long applicableStoreId = parseLong(request.getApplicableStoreId(), "applicable_store_id");
            if (applicableStoreId <= 0) {
                throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "applicable_store_id 必須為正整數");
            }

            // 建立 COMPENSATION 模板
            CouponTemplateEntity template = templateService.createCompensationTemplate(
                    adminAccountId, amount, applicableStoreId);

            // 寫入 coupon_target_accounts
            insertTargetAccounts(template.getTemplateId(), List.of(buyerId));

            responseObserver.onNext(AdminGrantCompensationCouponResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toTemplateInfo(template))
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminGrantCompensationCouponResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminGrantCompensationCouponResponse.newBuilder(), e, "adminGrantCompensationCoupon");
        }
    }

    @Override
    public void adminVoidInstance(AdminVoidInstanceRequest request,
                                  StreamObserver<AdminVoidInstanceResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminAccountId = GrpcAuthContext.requireAccountId();
            Long instanceId = parseLong(request.getInstanceId(), "instance_id");

            boolean voided = instanceService.voidInstance(instanceId, request.getReason(), adminAccountId);
            if (!voided) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_INVALID_STATUS);
            }

            responseObserver.onNext(AdminVoidInstanceResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message()).build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminVoidInstanceResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminVoidInstanceResponse.newBuilder(), e, "adminVoidInstance");
        }
    }

    @Override
    public void adminUpdateSellerQuota(AdminUpdateSellerQuotaRequest request,
                                        StreamObserver<AdminUpdateSellerQuotaResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long storeId = parseLong(request.getStoreId(), "store_id");
            int maxCount = parseInt(request.getMaxTemplateCount(), "max_template_count");

            quotaService.updateQuota(storeId, maxCount);

            responseObserver.onNext(AdminUpdateSellerQuotaResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message()).build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(AdminUpdateSellerQuotaResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, AdminUpdateSellerQuotaResponse.newBuilder(), e, "adminUpdateSellerQuota");
        }
    }

    // ==================== AfterSales Service 呼叫 ====================

    @Override
    public void grantSellerCompensationCoupon(GrantSellerCompensationCouponRequest request,
                                               StreamObserver<GrantSellerCompensationCouponResponse> responseObserver) {
        try {
            // 內部服務呼叫，不檢查角色（由 x-internal-service 自動識別）
            Long storeId = parseLong(request.getStoreId(), "store_id");
            Long buyerId = parseLong(request.getBuyerId(), "buyer_id");
            Long amount = parseLong(request.getAmount(), "amount");
            Long minOrderAmount = parseLong(request.getMinOrderAmount(), "min_order_amount");

            // 驗證集中化：補償券業務規則的唯一真實來源（依設計書 §5.12）
            // 上游 service（aftersales 等）不再重複驗證，避免規則散落 + 內部直呼繞過。
            if (amount <= 0) {
                throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "補償金額必須大於 0");
            }
            if (minOrderAmount <= 0) {
                throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "最低消費必須大於 0");
            }
            // 業務規則：最低消費 >= 面額 * 5、最低消費 - 面額 >= 100
            if (minOrderAmount < amount * 5 || minOrderAmount - amount < 100) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_RULE_VIOLATED);
            }
            String caseNo = request.getRefCaseNo();

            // 建立賣家補償券模板（target_type=COMPENSATION）
            CouponTemplateEntity template = templateService.createSellerCompensationTemplate(
                    storeId, amount, minOrderAmount, caseNo);

            // 寫入 coupon_target_accounts
            insertTargetAccounts(template.getTemplateId(), List.of(buyerId));

            SellerCompensationCouponData data = SellerCompensationCouponData.newBuilder()
                    .setTemplateId(String.valueOf(template.getTemplateId()))
                    .setAmount(String.valueOf(amount))
                    .setMinOrderAmount(String.valueOf(minOrderAmount))
                    .build();

            responseObserver.onNext(GrantSellerCompensationCouponResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (CouponBusinessException e) {
            responseObserver.onNext(GrantSellerCompensationCouponResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            onSystemError(responseObserver, GrantSellerCompensationCouponResponse.newBuilder(), e, "grantSellerCompensationCoupon");
        }
    }

    // ==================== 工具方法 ====================

    /**
     * 批次寫入 coupon_target_accounts
     */
    private void insertTargetAccounts(Long templateId, List<Long> accountIds) {
        for (Long accountId : accountIds) {
            targetAccountMapper.insertIgnore(templateId, accountId);
        }
    }

    /**
     * target_type 資格判定（§3.4 虛擬碼實作）
     */
    private boolean isEligible(CouponTemplateEntity tpl, long buyerId,
                                long buyerCreatedAt, Set<Long> memberSellerIds,
                                List<Long> designatedTemplateIds) {
        String targetType = tpl.getTargetType();
        if (targetType == null) return true; // 舊資料相容

        String allValue = CouponTargetType.ALL.value();
        String memberValue = CouponTargetType.MEMBER.value();
        String designatedValue = CouponTargetType.DESIGNATED.value();
        String compensationValue = CouponTargetType.COMPENSATION.value();

        if (allValue.equals(targetType)) {
            // 買家註冊時間 <= template.created_at
            if (buyerCreatedAt < 0) return false; // Account Service 呼叫失敗
            long templateCreatedAtMs = tpl.getCreatedAt() != null
                    ? tpl.getCreatedAt().toInstant(java.time.ZoneOffset.UTC).toEpochMilli()
                    : Long.MAX_VALUE;
            return buyerCreatedAt <= templateCreatedAtMs;
        } else if (memberValue.equals(targetType)) {
            // 買家在該賣家有完成交易紀錄
            Long sellerId = tpl.getApplicableStoreId();
            return sellerId != null && memberSellerIds.contains(sellerId);
        } else if (designatedValue.equals(targetType) || compensationValue.equals(targetType)) {
            // 在 coupon_target_accounts 表
            return designatedTemplateIds.contains(tpl.getTemplateId());
        } else {
            return true;
        }
    }

    private CouponTemplateEntity getTemplateWithCache(Long templateId) {
        CouponTemplateEntity cached = cacheService.getTemplate(templateId);
        if (cached != null) {
            return cached;
        }
        CouponTemplateEntity template = templateService.getById(templateId);
        if (template != null) {
            cacheService.putTemplate(template);
        }
        return template;
    }

    private TemplateInfo toTemplateInfo(CouponTemplateEntity tpl) {
        return toTemplateInfo(tpl, null);
    }

    private TemplateInfo toTemplateInfo(CouponTemplateEntity tpl, SellerMeta sellerMeta) {
        return toTemplateInfoWithClaim(tpl, sellerMeta,
                Boolean.TRUE.equals(tpl.getClaimRequired()), false);
    }

    private TemplateInfo toTemplateInfoWithClaim(CouponTemplateEntity tpl, SellerMeta sellerMeta,
                                                  boolean claimRequired, boolean claimed) {
        if (tpl == null) return TemplateInfo.getDefaultInstance();
        var builder = TemplateInfo.newBuilder()
                .setTemplateId(String.valueOf(tpl.getTemplateId()))
                .setTitle(tpl.getTitle())
                .setType(tpl.getType())
                .setIssuerType(tpl.getIssuerType())
                .setThresholdAmount(String.valueOf(tpl.getThresholdAmount()))
                .setUsageLimitPerAccount(String.valueOf(tpl.getUsageLimitPerAccount()))
                .setTotalIssueLimit(String.valueOf(tpl.getTotalIssueLimit()))
                .setIssuedCount(String.valueOf(tpl.getIssuedCount()))
                .setStatus(tpl.getStatus())
                .setStartAt(formatDateTime(tpl.getStartAt()))
                .setEndAt(formatDateTime(tpl.getEndAt()))
                .setCreatedAt(formatDateTime(tpl.getCreatedAt()))
                .setSellerName(sellerMeta != null ? sellerMeta.getSellerName() : "")
                .setSellerAvatarKey(sellerMeta != null ? sellerMeta.getLogoKey() : "")
                // 賣家頁 canonical 網址走 store_code；值與店名同在這一份 SellerMeta 裡
                .setStoreCode(sellerMeta != null ? sellerMeta.getStoreCode() : "")
                .setTargetType(tpl.getTargetType() != null ? tpl.getTargetType() : "")
                .setClaimRequired(claimRequired)
                .setClaimed(claimed);

        if (tpl.getIssuerAccountId() != null) {
            builder.setIssuerAccountId(String.valueOf(tpl.getIssuerAccountId()));
        }
        if (tpl.getIssuerStoreId() != null) {
            builder.setIssuerStoreId(String.valueOf(tpl.getIssuerStoreId()));
        }
        if (tpl.getApplicableStoreId() != null) {
            builder.setApplicableStoreId(String.valueOf(tpl.getApplicableStoreId()));
        }
        if (tpl.getDiscountAmount() != null) {
            builder.setDiscountAmount(String.valueOf(tpl.getDiscountAmount()));
        }
        if (tpl.getDiscountPercent() != null) {
            builder.setDiscountPercent(String.valueOf(tpl.getDiscountPercent()));
        }
        if (tpl.getMaxDiscountAmount() != null) {
            builder.setMaxDiscountAmount(String.valueOf(tpl.getMaxDiscountAmount()));
        }
        return builder.build();
    }

    private InstanceInfo toInstanceInfo(CouponInstanceEntity inst, CouponTemplateEntity tpl) {
        return toInstanceInfo(inst, tpl, null);
    }

    private InstanceInfo toInstanceInfo(CouponInstanceEntity inst, CouponTemplateEntity tpl, SellerMeta sellerMeta) {
        var builder = InstanceInfo.newBuilder()
                .setInstanceId(String.valueOf(inst.getInstanceId()))
                .setTemplateId(String.valueOf(inst.getTemplateId()))
                .setStatus(inst.getStatus())
                .setCreatedAt(formatDateTime(inst.getCreatedAt()))
                .setSellerName(sellerMeta != null ? sellerMeta.getSellerName() : "")
                .setSellerAvatarKey(sellerMeta != null ? sellerMeta.getLogoKey() : "")
                // 賣家頁 canonical 網址走 store_code；值與店名同在這一份 SellerMeta 裡
                .setStoreCode(sellerMeta != null ? sellerMeta.getStoreCode() : "");

        if (inst.getApplicableStoreId() != null) {
            builder.setApplicableStoreId(String.valueOf(inst.getApplicableStoreId()));
        }

        if (tpl != null) {
            builder.setTitle(tpl.getTitle())
                   .setType(tpl.getType())
                   .setThresholdAmount(String.valueOf(tpl.getThresholdAmount()))
                   .setStartAt(formatDateTime(tpl.getStartAt()))
                   .setEndAt(formatDateTime(tpl.getEndAt()));
            if (tpl.getDiscountAmount() != null) {
                builder.setDiscountAmount(String.valueOf(tpl.getDiscountAmount()));
            }
            if (tpl.getDiscountPercent() != null) {
                builder.setDiscountPercent(String.valueOf(tpl.getDiscountPercent()));
            }
            if (tpl.getMaxDiscountAmount() != null) {
                builder.setMaxDiscountAmount(String.valueOf(tpl.getMaxDiscountAmount()));
            }
        }
        return builder.build();
    }

    /**
     * 批量查詢賣家名稱（降級為空 Map）
     */
    private Map<Long, SellerMeta> batchGetSellerMeta(List<Long> storeIds) {
        return sellerGrpcClient.batchGetSellerMeta(storeIds);
    }

    /**
     * 從 sellerMetaMap 取出該賣家的 meta；全平台券（store_id 為 null）或聚合失敗時回 null，
     * 由轉換端一致降級成空字串。整包傳遞而非逐欄拆成平行參數，是為了讓「店名 / logo /
     * store_code 同源」這件事在型別上就成立——三個平行 String 參數很容易在新增欄位時漏傳。
     */
    private SellerMeta sellerMetaOf(Map<Long, SellerMeta> sellerMetaMap, Long storeId) {
        if (storeId == null || sellerMetaMap == null) {
            return null;
        }
        return sellerMetaMap.get(storeId);
    }

    private String formatDateTime(LocalDateTime dt) {
        return ProtoConversionUtils.formatIso8601(dt);
    }

    private LocalDateTime parseDateTimeRequired(String iso, String fieldName) {
        try {
            return ProtoConversionUtils.parseIso8601Required(iso, fieldName);
        } catch (IllegalArgumentException e) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM, e.getMessage());
        }
    }

    private Long parseLong(String value, String fieldName) {
        try {
            return ProtoConversionUtils.parseLongRequired(value, fieldName);
        } catch (IllegalArgumentException e) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM, e.getMessage());
        }
    }

    private Long parseNullableLong(String value) {
        return ProtoConversionUtils.parseLongOrNull(value);
    }

    private int parseInt(String value, String fieldName) {
        try {
            return ProtoConversionUtils.parseIntRequired(value, fieldName);
        } catch (IllegalArgumentException e) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM, e.getMessage());
        }
    }

    private Integer parseNullableInt(String value) {
        return ProtoConversionUtils.parseIntOrNull(value);
    }

    private int parseIntOrDefault(String value, int defaultValue) {
        return ProtoConversionUtils.parseIntOrDefault(value, defaultValue);
    }

    /**
     * 通用業務錯誤處理（適用於有 code/message 欄位的 Response）
     */
    @SuppressWarnings("unchecked")
    private <T extends com.google.protobuf.GeneratedMessageV3, B extends com.google.protobuf.GeneratedMessageV3.Builder<B>>
    void onBusinessError(StreamObserver<T> responseObserver, B builder, CouponBusinessException e) {
        try {
            var codeField = builder.getDescriptorForType().findFieldByName("code");
            var messageField = builder.getDescriptorForType().findFieldByName("message");
            if (codeField != null) builder.setField(codeField, e.code());
            if (messageField != null) builder.setField(messageField, e.getMessage());
            responseObserver.onNext((T) builder.build());
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(io.grpc.Status.INTERNAL.withDescription("系統錯誤").asRuntimeException());
        }
    }

    @SuppressWarnings("unchecked")
    private <T extends com.google.protobuf.GeneratedMessageV3, B extends com.google.protobuf.GeneratedMessageV3.Builder<B>>
    void onSystemError(StreamObserver<T> responseObserver, B builder, Exception e, String method) {
        GrpcErrorHandler.handleException(responseObserver, builder, e, "CouponGrpcService", method);
    }
}
