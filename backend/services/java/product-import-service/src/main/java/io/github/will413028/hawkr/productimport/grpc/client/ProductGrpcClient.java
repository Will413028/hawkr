package io.github.will413028.hawkr.productimport.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesRequest;
import io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalRequest;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse;
import io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalRequest;
import io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ProductServiceGrpc;
import io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalRequest;
import io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse;
import io.github.will413028.hawkr.product.grpc.VerifyCertificationRequest;
import io.github.will413028.hawkr.product.grpc.VerifyCertificationResponse;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

@Component
public class ProductGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ProductGrpcClient.class);
    private static final String TARGET = "product";

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     *
     * <p><b>10 秒怎麼來的</b>：掛這個值的三支（{@code validateLeafCategoryInternal} /
     * {@code batchCheckSellerProductNames} / {@code getProductsBriefInternal}）在 product-service
     * 內都只做<b>本地 DB 讀取</b>，p99 毫秒級；取值與 {@code order-service} 的
     * {@code ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC} 一致。
     *
     * <p>⚠️ 會在 product-service 內<b>再往外打一層</b>的那幾支不適用本值，見
     * {@link #NESTED_CALL_DEADLINE_SEC}。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    /**
     * issue #3332：<b>product-service 內部還會再往外打一層</b>的 RPC 專用 deadline（秒）。
     *
     * <p>外層 deadline 必須<b>大於</b>內層自己的預算，否則外層先到期 ＝ 把一個原本慢但會成功的
     * 請求硬切成 {@code DEADLINE_EXCEEDED}（#3230 就是這個形狀：給 media 呼叫套 10 秒被審查抓到）。
     * 三支的內層預算（實查，非推測）：
     * <ol>
     *   <li>{@code validateProductDraft} → {@code ProductPublishValidator.validate}
     *       → {@code ShippingGrpcClient.validateProductShipping}，內層 <b>10 秒</b>。</li>
     *   <li>{@code createProductFromImportInternal} → {@code ProductImportService.createProductFromImport}
     *       → 同一個 validator ⇒ 同樣內含那一支 <b>10 秒</b>的 shipping 呼叫，外加商品落盤寫入。</li>
     *   <li>{@code verifyCertification} → {@code GovCertificationService.verifyDetailed}
     *       → <b>政府開放資料 API</b>（同步路徑：connect-timeout 5 秒 ＋ read-timeout 10 秒
     *       ⇒ 上界 <b>15 秒</b>，無內部重試）。這是本 client 唯一會打第三方的一支。</li>
     * </ol>
     *
     * <p>取 20 秒＝三者中最大的內層上界（15 秒）再留 5 秒餘裕，同時沿用 repo 內既有的 20L 檔位
     * （各服務 MediaGrpcClient 的 {@code MEDIA_DEADLINE_SEC}），不另立新數字。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用。
     */
    public static final long NESTED_CALL_DEADLINE_SEC = 20L;

    private final GrpcStubFactory grpcStubFactory;

    public ProductGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    private ProductServiceGrpc.ProductServiceBlockingStub stub() {
        return grpcStubFactory.blocking(TARGET, ProductServiceGrpc::newBlockingStub);
    }

    public ValidateLeafCategoryInternalResponse validateLeafCategory(String categoryId) {
        try {
            return stub()
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .validateLeafCategoryInternal(
                    ValidateLeafCategoryInternalRequest.newBuilder()
                            .setCategoryId(categoryId)
                            .build());
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.validateLeafCategory.error",
                    "ValidateLeafCategoryInternal failed: " + e.getStatus(), e);
            throw e;
        }
    }

    public VerifyCertificationResponse verifyCertification(String certNo, String type) {
        try {
            return stub()
                    .withDeadlineAfter(NESTED_CALL_DEADLINE_SEC, TimeUnit.SECONDS)
                    .verifyCertification(VerifyCertificationRequest.newBuilder()
                    .setCertNo(certNo == null ? "" : certNo)
                    .setType(type == null ? "" : type)
                    .build());
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.verifyCertification.error",
                    "VerifyCertification failed: " + e.getStatus(), e);
            throw e;
        }
    }

    public BatchCheckSellerProductNamesResponse batchCheckSellerProductNames(long storeId,
                                                                             List<String> names) {
        try {
            return stub()
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .batchCheckSellerProductNames(
                    BatchCheckSellerProductNamesRequest.newBuilder()
                            .setStoreId(storeId)
                            .addAllNames(names)
                            .build());
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.batchCheckSellerProductNames.error",
                    "BatchCheckSellerProductNames failed: " + e.getStatus(), e);
            throw e;
        }
    }

    /**
     * 40 §4.3a：呼叫 product-service ValidateProductDraft（內部 RPC，x-internal-service 由 GrpcStubFactory 注入）。
     * 三時機點（CSV_PARSE / IMPORT_UPDATE / IMPORT_COMMIT）共用一支 RPC，context 決定規則放寬程度。
     */
    public ValidateProductDraftResponse validateProductDraft(long storeId, ProductDraft draft,
                                                              ValidateProductDraftRequest.ValidationContext context) {
        try {
            return stub()
                    .withDeadlineAfter(NESTED_CALL_DEADLINE_SEC, TimeUnit.SECONDS)
                    .validateProductDraft(ValidateProductDraftRequest.newBuilder()
                    .setStoreId(storeId)
                    .setDraft(draft)
                    .setContext(context)
                    .build());
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.validateProductDraft.error",
                    "ValidateProductDraft failed: " + e.getStatus(), e);
            throw e;
        }
    }

    /**
     * 40 §4.3b：batch 查正式商品 brief（COMMITTED item 的 formal_product_status 來源）。
     * 上限 200 ids；caller_store_id 僅供 product-service 審計 log 用（account_id 命名債正名，
     * Phase 2 Task 16 review 補漏——本服務唯一呼叫端傳入的一律是 store_id，非登入 account_id）。
     */
    public GetProductsBriefInternalResponse getProductsBriefInternal(long callerStoreId, java.util.List<String> productIds) {
        try {
            return stub()
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getProductsBriefInternal(GetProductsBriefInternalRequest.newBuilder()
                    .setCallerStoreId(callerStoreId)
                    .addAllProductIds(productIds)
                    .build());
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.getProductsBriefInternal.error",
                    "GetProductsBriefInternal failed: " + e.getStatus(), e);
            throw e;
        }
    }

    public CreateProductFromImportInternalResponse createProductFromImport(CreateProductFromImportInternalRequest req) {
        try {
            return stub()
                    .withDeadlineAfter(NESTED_CALL_DEADLINE_SEC, TimeUnit.SECONDS)
                    .createProductFromImportInternal(req);
        } catch (StatusRuntimeException e) {
            log.error("ProductGrpcClient.createProductFromImport.error",
                    "CreateProductFromImportInternal failed: " + e.getStatus(), e);
            throw e;
        }
    }
}
