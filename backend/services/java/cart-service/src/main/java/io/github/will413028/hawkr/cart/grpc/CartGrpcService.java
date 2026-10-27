package io.github.will413028.hawkr.cart.grpc;

import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules;
import io.github.will413028.hawkr.cart.grpc.AddItemRequest;
import io.github.will413028.hawkr.cart.grpc.AddItemResponse;
import io.github.will413028.hawkr.cart.grpc.GetCartRequest;
import io.github.will413028.hawkr.cart.grpc.GetCartResponse;
import io.github.will413028.hawkr.cart.grpc.CartSellerGroup;
import io.github.will413028.hawkr.cart.grpc.CartItemView;
import io.github.will413028.hawkr.cart.grpc.MergeCartRequest;
import io.github.will413028.hawkr.cart.grpc.MergeCartResponse;
import io.github.will413028.hawkr.cart.grpc.SetItemQuantityRequest;
import io.github.will413028.hawkr.cart.grpc.SetItemQuantityResponse;
import io.github.will413028.hawkr.cart.grpc.CheckoutLockRequest;
import io.github.will413028.hawkr.cart.grpc.CheckoutLockResponse;
import io.github.will413028.hawkr.cart.grpc.CheckoutUnlockRequest;
import io.github.will413028.hawkr.cart.grpc.CheckoutUnlockResponse;
import io.github.will413028.hawkr.cart.grpc.RemoveItemsByCheckoutIdRequest;
import io.github.will413028.hawkr.cart.grpc.RemoveItemsByCheckoutIdResponse;
import io.github.will413028.hawkr.cart.grpc.CartServiceGrpc;
import io.github.will413028.hawkr.cart.exception.CartBusinessException;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.cart.service.CartService;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.product.grpc.SkuCartInfo;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * Cart Service gRPC 實作（設計書 §5 全 7 個 API）
 */
@GrpcService
public class CartGrpcService extends CartServiceGrpc.CartServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(CartGrpcService.class);
    private final CartService cartService;

    public CartGrpcService(CartService cartService) {
        this.cartService = cartService;
    }

    // ==================== AddItem ====================

    @Override
    public void addItem(AddItemRequest request, StreamObserver<AddItemResponse> responseObserver) {
        try {
            OwnerInfo owner = resolveOwner(request.getGuestId());
            Long skuId = parseLong(request.getSkuId(), "sku_id");
            int quantity = parseInt(request.getQuantity(), "quantity");

            if (quantity <= 0) {
                throw Status.INVALID_ARGUMENT.withDescription("quantity 必須大於 0").asRuntimeException();
            }

            CartService.AddItemResult result = cartService.addItem(
                    owner.buyerId(), owner.guestId(), skuId, quantity);

            responseObserver.onNext(AddItemResponse.newBuilder()
                    .setCode(result.code())
                    .setMessage(result.message())
                    .setTotalItems(String.valueOf(result.totalItems()))
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(AddItemResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, AddItemResponse.newBuilder(), e, "CartGrpcService", "addItem");
        }
    }

    // ==================== GetCart ====================

    @Override
    public void getCart(GetCartRequest request, StreamObserver<GetCartResponse> responseObserver) {
        try {
            OwnerInfo owner = resolveOwner(request.getGuestId());

            CartService.GetCartResult result = cartService.getCart(owner.buyerId(), owner.guestId());

            var responseBuilder = GetCartResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setEstimatedTotalAmount(String.valueOf(result.estimatedTotal()))
                    .setTotalItems(String.valueOf(result.totalItems()));

            Map<String, SkuCartInfo> skuInfoMap = result.skuCartInfoMap() != null
                    ? result.skuCartInfoMap() : Collections.emptyMap();

            for (CartService.SellerGroup group : result.groups()) {
                // 49 §6.6：服務側只填 objectKey，顯示 URL 由 Gateway 組（logo_key enrich 自 SellerMeta，S9 過渡期值可能仍為 URL，Gateway BuildURL passthrough）
                var groupBuilder = CartSellerGroup.newBuilder()
                        .setStoreId(String.valueOf(group.storeId()))
                        .setSellerName(group.sellerName())
                        .setLogoKey(group.logoUrl() != null ? group.logoUrl() : "")
                        .setSellerCity(group.city() != null ? group.city() : "")
                        .setSellerDistrict(group.district() != null ? group.district() : "")
                        .setStoreCode(group.storeCode() != null ? group.storeCode() : "");

                for (CartItemEntity item : group.items()) {
                    String imageKey = item.getImageObjectKey() != null ? item.getImageObjectKey() : "";
                    String skuIdStr = String.valueOf(item.getSkuId());
                    SkuCartInfo skuInfo = skuInfoMap.get(skuIdStr);

                    var itemBuilder = CartItemView.newBuilder()
                            .setSkuId(skuIdStr)
                            .setQuantity(String.valueOf(item.getQuantity()))
                            .setName(item.getSkuNameSnapshot())
                            .setPrice(String.valueOf(item.getAddedUnitPrice()))      // 實際成交單價（特價優先）
                            .setListPrice(String.valueOf(item.getAddedListPrice()))  // 原價快照
                            .setStatus(item.getStatus())
                            .setProductId(String.valueOf(item.getProductId()))
                            .setSpecName(item.getSpecNameSnapshot() != null ? item.getSpecNameSnapshot() : "")
                            .setImageKey(imageKey);

                    // 補充即時庫存+配送溫層+可購買性（來自 Product Service 批次查詢）
                    // 判定鏈：LOOKUP_FAILED (null) → DELISTED (!is_active) → OUT_OF_STOCK (stock<=0)
                    //        → INSUFFICIENT_STOCK (stock<qty) → OK
                    // 詳見 ProductPurchasabilityRules + 04 §5.2 + 20 §4.14
                    ProductPurchasabilityRules.Purchasability purchasability;
                    if (skuInfo == null) {
                        itemBuilder.setStockQty("0");
                        purchasability = ProductPurchasabilityRules.lookupFailed();
                    } else {
                        long stockQty = parseLongOrZero(skuInfo.getStockQty());
                        itemBuilder.setStockQty(skuInfo.getStockQty())
                                .addAllDeliveryTemps(skuInfo.getDeliveryTempsList());
                        purchasability = ProductPurchasabilityRules.evaluate(
                                skuInfo.getIsActive(),
                                stockQty,
                                item.getQuantity() != null ? item.getQuantity() : 0L);
                    }
                    itemBuilder.setPurchasable(purchasability.purchasable())
                            .setUnavailableReason(purchasability.reason());

                    groupBuilder.addItems(itemBuilder.build());
                }

                responseBuilder.addGroups(groupBuilder.build());
            }

            responseObserver.onNext(responseBuilder.build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(GetCartResponse.newBuilder()
                    .setCode(e.code())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetCartResponse.newBuilder(), e, "CartGrpcService", "getCart");
        }
    }

    // ==================== MergeCart ====================

    @Override
    public void mergeCart(MergeCartRequest request, StreamObserver<MergeCartResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可合併購物車
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = GrpcAuthContext.requireAccountId();

            String guestId = request.getGuestId();
            if (guestId == null || guestId.isEmpty()) {
                throw Status.INVALID_ARGUMENT.withDescription("guest_id 不可為空").asRuntimeException();
            }

            CartService.MergeCartResult result = cartService.mergeCart(buyerId, guestId);

            responseObserver.onNext(MergeCartResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setRemovedSelfOwnedCount(String.valueOf(result.removedSelfOwnedCount()))
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(MergeCartResponse.newBuilder()
                    .setCode(e.code())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, MergeCartResponse.newBuilder(), e, "CartGrpcService", "mergeCart");
        }
    }

    // ==================== SetItemQuantity ====================

    @Override
    public void setItemQuantity(SetItemQuantityRequest request, StreamObserver<SetItemQuantityResponse> responseObserver) {
        try {
            OwnerInfo owner = resolveOwner(request.getGuestId());
            Long skuId = parseLong(request.getSkuId(), "sku_id");
            int quantity = parseInt(request.getQuantity(), "quantity");

            if (quantity < 0) {
                throw Status.INVALID_ARGUMENT.withDescription("quantity 不可為負數").asRuntimeException();
            }

            CartService.SetItemResult result = cartService.setItemQuantity(
                    owner.buyerId(), owner.guestId(), skuId, quantity);

            responseObserver.onNext(SetItemQuantityResponse.newBuilder()
                    .setCode(result.code())
                    .setMessage(result.message())
                    .setTotalItems(String.valueOf(result.totalItems()))
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(SetItemQuantityResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, SetItemQuantityResponse.newBuilder(), e, "CartGrpcService", "setItemQuantity");
        }
    }

    // ==================== CheckoutLock ====================

    @Override
    public void checkoutLock(CheckoutLockRequest request, StreamObserver<CheckoutLockResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可結帳鎖定
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = GrpcAuthContext.requireAccountId();

            String checkoutId = request.getCheckoutId();
            if (checkoutId == null || checkoutId.isEmpty()) {
                throw Status.INVALID_ARGUMENT.withDescription("checkout_id 不可為空").asRuntimeException();
            }

            List<Long> skuIds = request.getSkuIdsList().stream()
                    .map(s -> parseLong(s, "sku_id"))
                    .collect(Collectors.toList());

            int code = cartService.checkoutLock(buyerId, checkoutId, skuIds);

            responseObserver.onNext(CheckoutLockResponse.newBuilder()
                    .setCode(code)
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(CheckoutLockResponse.newBuilder()
                    .setCode(e.code())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutLockResponse.newBuilder(), e, "CartGrpcService", "checkoutLock");
        }
    }

    // ==================== CheckoutUnlock ====================

    @Override
    public void checkoutUnlock(CheckoutUnlockRequest request, StreamObserver<CheckoutUnlockResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可解除鎖定
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = GrpcAuthContext.requireAccountId();

            String checkoutId = request.getCheckoutId();
            if (checkoutId == null || checkoutId.isEmpty()) {
                throw Status.INVALID_ARGUMENT.withDescription("checkout_id 不可為空").asRuntimeException();
            }

            List<Long> skuIds = request.getSkuIdsList().stream()
                    .map(s -> parseLong(s, "sku_id"))
                    .collect(Collectors.toList());

            int code = cartService.checkoutUnlock(buyerId, checkoutId, skuIds.isEmpty() ? null : skuIds);

            responseObserver.onNext(CheckoutUnlockResponse.newBuilder()
                    .setCode(code)
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(CheckoutUnlockResponse.newBuilder()
                    .setCode(e.code())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutUnlockResponse.newBuilder(), e, "CartGrpcService", "checkoutUnlock");
        }
    }

    // ==================== RemoveItemsByCheckoutId ====================

    @Override
    public void removeItemsByCheckoutId(RemoveItemsByCheckoutIdRequest request,
                                         StreamObserver<RemoveItemsByCheckoutIdResponse> responseObserver) {
        try {
            Long buyerId = GrpcAuthContext.getAccountId();
            if (buyerId == null) {
                throw Status.UNAUTHENTICATED.withDescription("移除購物車項目需要登入").asRuntimeException();
            }

            String checkoutId = request.getCheckoutId();
            if (checkoutId == null || checkoutId.isEmpty()) {
                throw Status.INVALID_ARGUMENT.withDescription("checkout_id 不可為空").asRuntimeException();
            }

            List<Long> skuIds = request.getSkuIdsList().stream()
                    .map(s -> parseLong(s, "sku_id"))
                    .collect(Collectors.toList());

            int code = cartService.removeItemsByCheckoutId(buyerId, checkoutId, skuIds.isEmpty() ? null : skuIds);

            responseObserver.onNext(RemoveItemsByCheckoutIdResponse.newBuilder()
                    .setCode(code)
                    .build());
            responseObserver.onCompleted();
        } catch (CartBusinessException e) {
            responseObserver.onNext(RemoveItemsByCheckoutIdResponse.newBuilder()
                    .setCode(e.code())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, RemoveItemsByCheckoutIdResponse.newBuilder(), e, "CartGrpcService", "removeItemsByCheckoutId");
        }
    }

    // ==================== 工具方法 ====================

    /**
     * 解析 owner 身分（設計書 §1.2.1）
     * - metadata 有 account_id → member flow
     * - 無 account_id → guest flow（需要 guest_id）
     */
    private OwnerInfo resolveOwner(String guestId) {
        Long buyerId = GrpcAuthContext.getAccountId();
        if (buyerId != null) {
            return new OwnerInfo(buyerId, null);
        }
        // guest flow
        if (guestId == null || guestId.isEmpty()) {
            throw Status.UNAUTHENTICATED
                    .withDescription("未提供有效身分（缺少 account_id 和 guest_id）")
                    .asRuntimeException();
        }
        return new OwnerInfo(null, guestId);
    }

    private Long parseLong(String value, String fieldName) {
        return ProtoConversionUtils.parseLongRequired(value, fieldName);
    }

    private int parseInt(String value, String fieldName) {
        return ProtoConversionUtils.parseIntRequired(value, fieldName);
    }

    /** SkuCartInfo.stock_qty 是 string；ProductPurchasabilityRules 簽章為 long，這裡轉換。空 / 非數字一律回 0 視為缺貨。 */
    private static long parseLongOrZero(String value) {
        if (value == null || value.isEmpty()) {
            return 0L;
        }
        try {
            return Long.parseLong(value);
        } catch (NumberFormatException e) {
            return 0L;
        }
    }

    private record OwnerInfo(Long buyerId, String guestId) {}
}
