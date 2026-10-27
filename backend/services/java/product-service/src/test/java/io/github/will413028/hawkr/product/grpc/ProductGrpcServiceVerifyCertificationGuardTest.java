package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.product.service.GovCertificationService;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.ProductService;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code verifyCertification} 的 cert_no 字元守衛。
 *
 * <p><b>它守的是什麼</b>：certNo 一路從 HTTP query 傳到
 * {@code GovCertificationService.buildGovApiUrl}，在那裡<b>原字未編碼</b>串進政府 API 的查詢
 * 字串，後面緊接著 {@code &api_key=<真實金鑰>}。修這條之前，入口只擋空字串，因此只要送一個含
 * URI 保留字元的 cert_no，{@code URI.create} 就會丟出訊息內嵌整條 URL（金鑰在內）的例外。
 * 這是唯一一條不需要任何外部服務先壞掉、呼叫端自己就能反覆觸發的外洩路徑。
 *
 * <p><b>為什麼斷言「沒有呼叫 govCertificationService」</b>：只看回應碼的話，「先送出去再擋」
 * 的實作照樣會通過——而外洩發生在送出去的那一刻，回應碼是什麼已經無關。
 *
 * <p>白名單規則本身（字元集與長度）在 {@code GovCertificationServiceTest}，gateway 側的同一組
 * 規則在 {@code gateway/internal/handler/product_test.go}；三處必須一致。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
@DisplayName("ProductGrpcService.verifyCertification: cert_no 字元守衛")
class ProductGrpcServiceVerifyCertificationGuardTest {

    private static final long STORE_ID = 2685914682L;

    @Mock private ProductService productService;
    @Mock private ProductProtoConverter protoConverter;
    @Mock private io.github.will413028.hawkr.product.service.ProductCacheService cacheService;
    @Mock private io.github.will413028.hawkr.product.service.CategoryCacheService categoryCacheService;
    @Mock private org.springframework.data.redis.core.StringRedisTemplate redisTemplate;
    @Mock private io.github.will413028.hawkr.product.service.StockService stockService;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient shippingGrpcClient;
    @Mock private ProductSkuRepository skuRepository;
    @Mock private GovCertificationService govCertificationService;
    @Mock private io.github.will413028.hawkr.product.service.HashtagService hashtagService;
    @Mock private io.github.will413028.hawkr.product.service.PopularHashtagService popularHashtagService;
    @Mock private io.github.will413028.hawkr.product.service.ProductImportService productImportService;
    @Mock private io.github.will413028.hawkr.product.service.validator.ProductPublishValidator productPublishValidator;
    @Mock private io.github.will413028.hawkr.product.repository.ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService;
    @Mock io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter;

    private Context previousContext;

    @BeforeEach
    void attachSellerContext() {
        previousContext = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, STORE_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.SELLER.value())
                .attach();

        // 守衛之後的 happy path 也 stub 齊：守衛被拿掉時請求要能一路跑到底並回 code 0，
        // 紅字才會指向真正的後果（爛 cert_no 真的被送去組 URL 了）。
        // §9.4：handler 改走 verifyDetailed()（結構化結果是 validity 的唯一載體），
        // exists 由 found && validity==VALID 導出，不再是獨立的 boolean 呼叫。
        when(govCertificationService.verifyDetailed(anyString(), any())).thenAnswer(inv ->
                new GovCertificationService.GovVerifyResult(
                        CertificationType.ORGANIC.value(), inv.getArgument(0), "2026-08-26T00:00:00Z",
                        true, GovCertificationService.Validity.VALID, "通過", "2028-10-14",
                        "○○農場", "台南市", "○○驗證公司", "小松菜", 1, null));
    }

    @AfterEach
    void detachSellerContext() {
        Context.current().detach(previousContext);
    }

    private ProductGrpcService buildService() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    private VerifyCertificationResponse call(String certNo) {
        return call(certNo, CertificationType.ORGANIC);
    }

    private VerifyCertificationResponse call(String certNo, CertificationType certType) {
        @SuppressWarnings("unchecked")
        StreamObserver<VerifyCertificationResponse> observer = mock(StreamObserver.class);
        buildService().verifyCertification(
                VerifyCertificationRequest.newBuilder()
                        .setCertNo(certNo)
                        .setType(certType.value())
                        .build(),
                observer);

        ArgumentCaptor<VerifyCertificationResponse> captor =
                ArgumentCaptor.forClass(VerifyCertificationResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        return captor.getValue();
    }

    @ParameterizedTest
    @ValueSource(strings = {
            "1-008 131005",       // 空白：URI.create 直接丟含整條 URL 的例外
            "1-008-131005<x>",    // 角括號
            "abc{}",              // 大括號
            "abc|def",            // 直線
            "abc^def",            // caret
            "%41",                // 裸百分號
            "abc&api_key=stolen", // 想在查詢字串裡多塞一個參數
            "有機認證",             // 非 ASCII
    })
    @DisplayName("含非白名單字元的 cert_no → INVALID_PARAM，且絕不能已經送去組 URL")
    void invalidCertNoIsRejectedBeforeReachingGovApi(String certNo) {
        VerifyCertificationResponse response = call(certNo);

        // 這一條先斷言：外洩發生在送出去的那一刻。
        verify(govCertificationService, never()).verifyDetailed(anyString(), any());

        assertEquals(GrpcResponseCode.INVALID_PARAM.code(), response.getCode());
        assertTrue(response.getMessage().contains("認證編號"),
                "訊息要指出是認證編號這一欄，實際：" + response.getMessage());
    }

    @Test
    @DisplayName("超出長度上限的 cert_no 同樣擋下")
    void overlongCertNoIsRejected() {
        String certNo = "A".repeat(
                io.github.will413028.hawkr.product.service.GovCertificationService.CERT_NO_MAX_LENGTH + 1);

        VerifyCertificationResponse response = call(certNo);

        verify(govCertificationService, never()).verifyDetailed(anyString(), any());
        assertEquals(GrpcResponseCode.INVALID_PARAM.code(), response.getCode());
    }

    @Test
    @DisplayName("空 cert_no 維持既有的 CERT_NO_REQUIRED，不被新守衛蓋掉")
    void blankCertNoKeepsExistingCode() {
        VerifyCertificationResponse response = call("   ");

        verify(govCertificationService, never()).verifyDetailed(anyString(), any());
        assertEquals(GrpcResponseCode.CERT_NO_REQUIRED.code(), response.getCode(),
                "空值有專屬錯誤碼 3001，不該被 400 取代");
    }

    @ParameterizedTest
    @ValueSource(strings = {"1-008-131005", "TAP-2025-0032", "CAS-2025-1234", "ALI-TEA-2026-015", "abc_123"})
    @DisplayName("合法 cert_no 仍照常送去驗證（守衛不得過度收緊）")
    void validCertNoStillPassesThrough(String certNo) {
        VerifyCertificationResponse response = call(certNo);

        verify(govCertificationService).verifyDetailed(certNo, CertificationType.ORGANIC);
        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertTrue(response.getExists());
    }

    @ParameterizedTest
    @CsvSource({
            "HACCP, HACCP 2026 DP 0047",
            "ISO,   ISO 22000:2018",
            "GMP,   GMP/2026/0031",
    })
    @DisplayName("不可線上驗證的類型：帶特殊字元的 cert_no 仍回 3003，不被白名單改判成 400")
    void nonVerifiableTypeIsNotRejectedByWhitelist(String type, String certNo) {
        VerifyCertificationResponse response = call(certNo, CertificationType.fromCode(type));

        // 這三型的 certNo 從不離開系統（下面這條斷言就是理由），所以白名單對它們零安全效益；
        // 擺在類型判斷之前只會把 `ISO 22000:2018` 這種常見合法編號誤判成 400，
        // 無故改掉設計書 19 §13 測試計畫第 4 條的對外契約。
        verify(govCertificationService, never()).verifyDetailed(anyString(), any());

        assertEquals(GrpcResponseCode.CERT_TYPE_NOT_VERIFIABLE.code(), response.getCode(),
                "應維持「此認證類型暫不支援線上驗證」，實際訊息：" + response.getMessage());
    }
}
