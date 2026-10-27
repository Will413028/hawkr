package io.github.will413028.hawkr.product.service;

import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.classic.spi.IThrowableProxy;
import ch.qos.logback.core.read.ListAppender;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.product.config.GovApiProperties;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;
import org.springframework.test.util.ReflectionTestUtils;

import java.net.http.HttpClient;
import java.net.http.HttpResponse;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 農業部認證查詢：cert_no 字元白名單，以及例外處理不得外洩帶 api_key 的 URL。
 *
 * <p>{@code buildGovApiUrl} 把 {@code certNo} 原字未編碼串進查詢字串，後面再接
 * {@code &api_key=<真實金鑰>}。只要 {@code certNo} 含 URI 保留字元（空白、{@code < > { } | ^}
 * 或裸 {@code %}），{@code URI.create(url)} 就會丟 {@code IllegalArgumentException}，而該例外的
 * 訊息內嵌整條 URL——金鑰在內。這條路徑不需要任何外部服務先壞掉，呼叫端自己挑一個 cert_no
 * 就能重複觸發。
 *
 * <p>兩道防線各自對應一組測試：
 * <ul>
 *   <li>入口白名單：非法字元在打政府 API 之前就被擋掉，連 circuit breaker 都不該被誤觸。</li>
 *   <li>catch-all 分支：即使日後有別種例外落進同一個 catch，也只記例外類別名稱，不把整個
 *       例外物件交給 logger。</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
@DisplayName("GovCertificationService: cert_no 白名單與例外不外洩 api_key")
class GovCertificationServiceTest {

    /** 假金鑰，只用來斷言「這串字不得出現在 log 裡」。 */
    private static final String API_KEY = "TESTAPIKEYFAKEVALUE7Q4W8E2R6T0Y5"; // gitleaks:allow（測試用的假 key）

    @Mock
    private StringRedisTemplate redisTemplate;

    @Mock
    private ValueOperations<String, String> valueOps;

    @Mock
    private HttpClient httpClient;

    private GovApiProperties props;
    private GovCertificationService service;
    private Logger logbackLogger;
    private ListAppender<ILoggingEvent> appender;

    @BeforeEach
    void setUp() {
        props = new GovApiProperties();
        props.setApiKey1(API_KEY);

        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        when(valueOps.get(anyString())).thenReturn(null); // 快取與 circuit 都不命中

        service = new GovCertificationService(props, redisTemplate);
        // GovCertificationService 是 @Service，Spring 靠「唯一建構子」自動注入，因此不為了
        // 測試多開一個建構子（多一個就會變成沒有 @Autowired 的多建構子，啟動即失敗）。
        // 這裡改用 ReflectionTestUtils 換掉內建的 HttpClient。
        ReflectionTestUtils.setField(service, "httpClient", httpClient);

        logbackLogger = (Logger) LoggerFactory.getLogger(GovCertificationService.class);
        appender = new ListAppender<>();
        appender.start();
        logbackLogger.addAppender(appender);
    }

    @AfterEach
    void tearDown() {
        logbackLogger.detachAppender(appender);
        appender.stop();
    }

    /** 把 log 事件的訊息與整條 throwable chain 攤平，模擬 log 檔實際會落地的內容。 */
    private String capturedLogText() {
        StringBuilder sb = new StringBuilder();
        for (ILoggingEvent event : appender.list) {
            sb.append(event.getFormattedMessage()).append('\n');
            IThrowableProxy t = event.getThrowableProxy();
            while (t != null) {
                sb.append(t.getClassName()).append(": ").append(t.getMessage()).append('\n');
                t = t.getCause();
            }
        }
        return sb.toString();
    }

    // ==================== (a) 入口白名單 ====================

    @ParameterizedTest
    @ValueSource(strings = {
            "1-008-131005",      // 有機（設計書 §7.1 與 seed data 實際樣本）
            "1-009-300022",      // 有機（設計書 §3 時序圖樣本）
            "TAP-2025-0032",     // 產銷履歷
            "CAS-2025-1234",     // CAS
            "ALI-TEA-2026-015",  // seed data 產地證明
            "abc_123",           // 底線
            "A",                 // 單字元下界
    })
    @DisplayName("合法 cert_no（英數 / 連字號 / 底線）一律放行")
    void acceptsWhitelistedCertNo(String certNo) {
        assertTrue(GovCertificationService.isValidCertNo(certNo), "應視為合法：" + certNo);
    }

    @ParameterizedTest
    @ValueSource(strings = {
            "1-008 131005",       // 空白 → URI.create 直接爆
            "1-008-131005<x>",    // 角括號
            "abc{}",              // 大括號
            "abc|def",            // 直線
            "abc^def",            // caret
            "%41",                // 裸百分號
            "abc&api_key=stolen", // 塞第二個 api_key 參數
            "abc?x=1",            // 塞問號
            "abc#frag",           // fragment
            "有機認證",             // 非 ASCII
            "abc/def",            // 路徑分隔
            "",                   // 空字串
            "   ",                // 全空白
    })
    @DisplayName("含非白名單字元的 cert_no 一律拒絕")
    void rejectsNonWhitelistedCertNo(String certNo) {
        assertFalse(GovCertificationService.isValidCertNo(certNo), "應視為不合法：" + certNo);
    }

    @Test
    @DisplayName("null 與超過長度上限的 cert_no 也要拒絕")
    void rejectsNullAndOverlongCertNo() {
        assertFalse(GovCertificationService.isValidCertNo(null));
        assertTrue(GovCertificationService.isValidCertNo("A".repeat(GovCertificationService.CERT_NO_MAX_LENGTH)));
        assertFalse(GovCertificationService.isValidCertNo("A".repeat(GovCertificationService.CERT_NO_MAX_LENGTH + 1)));
    }

    @Test
    @DisplayName("非法 cert_no 在打政府 API 之前就被擋下，且不得誤觸 circuit breaker")
    void invalidCertNoRejectedBeforeCallingGovApi() throws Exception {
        assertThrows(IllegalArgumentException.class,
                () -> service.verify("1-008-131005<x>", CertificationType.ORGANIC));

        verify(httpClient, never()).send(any(), any());
        // openCircuit 與快取寫回共用同一個 4 參數 set()：一次都不該被呼叫。
        // 修前這條路徑會走進 catch-all → openCircuit，等於任何人送一個爛 cert_no
        // 就能讓該認證類型的驗證停擺 60 秒。
        verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
    }

    @Test
    @DisplayName("非法 cert_no 被拒時，例外訊息本身不得帶出 api_key")
    void rejectionMessageDoesNotLeakApiKey() {
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class,
                () -> service.verify("1-008 131005", CertificationType.ORGANIC));
        assertFalse(String.valueOf(ex.getMessage()).contains(API_KEY),
                "拒絕訊息外洩 api_key：" + ex.getMessage());
        assertFalse(capturedLogText().contains(API_KEY),
                "log 外洩 api_key：" + capturedLogText());
    }

    // ==================== (b) catch-all 分支縱深防禦 ====================

    @Test
    @DisplayName("catch-all 分支：底層例外訊息含 api_key 時，不得把整個例外物件交給 logger")
    void catchAllMustNotLeakApiKey() throws Exception {
        // 真實觸發者是 URI.create(url) 的 IllegalArgumentException——訊息內嵌整條含
        // api_key 的 URL。這裡直接注入同型例外，避免測試綁死特定 JDK 的訊息措辭；
        // 白名單擋掉這個入口之後，這條分支仍要對其他未列舉的例外保持不外洩。
        String leakyMessage = "Illegal character in query at index 60: "
                + props.getOrganicUrl() + "?CertOrganicSn=bad&$top=1&api_key=" + API_KEY;
        when(httpClient.send(any(), any())).thenThrow(new IllegalArgumentException(leakyMessage));

        assertThrows(GovCertificationService.GovApiUnavailableException.class,
                () -> service.verify("1-008-131005", CertificationType.ORGANIC));

        String logged = capturedLogText();
        assertFalse(logged.contains(API_KEY), "catch-all 把含 api_key 的例外寫進 log：" + logged);
        assertTrue(logged.contains("IllegalArgumentException"),
                "仍需保留例外類別名稱供診斷，實際：" + logged);
    }

    @Test
    @DisplayName("catch-all 仍維持既有語意：轉成 GovApiUnavailableException 並觸發 circuit breaker")
    void catchAllStillOpensCircuitAndThrowsUnavailable() throws Exception {
        when(httpClient.send(any(), any())).thenThrow(new IllegalStateException("boom"));

        assertThrows(GovCertificationService.GovApiUnavailableException.class,
                () -> service.verify("1-008-131005", CertificationType.ORGANIC));

        verify(valueOps).set(anyString(), anyString(), anyLong(), any()); // openCircuit
    }

    // ==================== 既有正常路徑不得被白名單擋掉 ====================

    /**
     * ⚠️ fixture 必須帶 {@code Status} 與 {@code EffectiveDate}：查驗語意已收窄成
     * 「只有有效狀態才算 VERIFIED」（設計書 19 §8.6），只有編號回聲的 body 會落成
     * {@code UNKNOWN}。三支的推導規則、已失效證書、以及三道防線都在
     * {@code GovCertificationServiceDetailedTest}；這裡只確認白名單沒有把正常路徑擋掉。
     */
    @Test
    @DisplayName("合法 cert_no 正常查到有效資料時仍回傳 true")
    @SuppressWarnings("unchecked")
    void validCertNoStillReachesGovApi() throws Exception {
        HttpResponse<String> response = mock(HttpResponse.class);
        when(response.statusCode()).thenReturn(200);
        when(response.body()).thenReturn("{\"RS\":\"OK\",\"Data\":[{\"CertOrganicSn\":\"1-008-131005\","
                + "\"Status\":\"通過\",\"EffectiveDate\":\"2099/12/31\"}],\"Next\":false}");
        // send() 是泛型方法，when(...) 推不出 HttpResponse<String>，改用 doReturn 繞開型別推導
        doReturn(response).when(httpClient).send(any(), any());

        assertTrue(service.verify("1-008-131005", CertificationType.ORGANIC));
    }

    /**
     * 同一組 fixture 只差 {@code Status}：這一支證明上面那支不是「隨便給什麼都會過」。
     *
     * <p>實測 {@code 1-014-123911} 的狀態就是「結束」，而舊版 {@code parseResponse} 只判資料陣列
     * 非空，照樣回 {@code exists=true} ——已失效的證書被顯示成「已驗證」（§8.6 要修的既有缺陷）。
     */
    @Test
    @DisplayName("Status=結束（已失效）→ 不算 VERIFIED，即使查得到資料")
    @SuppressWarnings("unchecked")
    void terminatedCertIsNotVerified() throws Exception {
        HttpResponse<String> response = mock(HttpResponse.class);
        when(response.statusCode()).thenReturn(200);
        when(response.body()).thenReturn("{\"RS\":\"OK\",\"Data\":[{\"CertOrganicSn\":\"1-008-131005\","
                + "\"Status\":\"結束\",\"EffectiveDate\":\"2099/12/31\"}],\"Next\":false}");
        doReturn(response).when(httpClient).send(any(), any());

        assertFalse(service.verify("1-008-131005", CertificationType.ORGANIC));
    }
}
