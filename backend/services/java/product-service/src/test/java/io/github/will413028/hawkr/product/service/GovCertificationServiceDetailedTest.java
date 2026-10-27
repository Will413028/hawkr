package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.product.config.GovApiProperties;
import io.github.will413028.hawkr.product.service.GovCertificationService.GovVerifyResult;
import io.github.will413028.hawkr.product.service.GovCertificationService.Validity;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;
import org.springframework.test.util.ReflectionTestUtils;

import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.startsWith;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code GovCertificationService.verifyDetailed} 的三道防線、validity 推導與快取分級（設計書 19 §8）。
 *
 * <p>白名單／api_key 不外洩那兩組在 {@link GovCertificationServiceTest}；本檔只管查驗語意。
 *
 * <p><b>每一支測試都必須答得出「它抓得到什麼」</b>，因此每個 case 的註解都指出「這條防線被拿掉時
 * 會發生什麼事」——那才是它守住的東西，不是「跑過一遍」。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
@DisplayName("GovCertificationService: 三道防線 / validity 推導 / 結構化快取")
class GovCertificationServiceDetailedTest {

    private static final String ORGANIC_NO = "1-009-300022";
    private static final String TAP_NO = "00100950250986";
    private static final String CAS_NO = "010105";

    /** 遠期／遠古日期：讓效期斷言與執行當下的時鐘、時區完全無關。 */
    private static final String FUTURE = "2099/12/31";
    private static final String PAST = "2000/01/01";

    @Mock private StringRedisTemplate redisTemplate;
    @Mock private ValueOperations<String, String> valueOps;
    @Mock private HttpClient httpClient;

    private GovApiProperties props;
    private GovCertificationService service;

    @BeforeEach
    void setUp() {
        props = new GovApiProperties();
        props.setApiKey1("TESTAPIKEYFAKEVALUE7Q4W8E2R6T0Y5");

        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        when(valueOps.get(anyString())).thenReturn(null); // 預設：快取與 circuit 都不命中

        service = new GovCertificationService(props, redisTemplate);
        ReflectionTestUtils.setField(service, "httpClient", httpClient);
    }

    // ==================== fixtures ====================

    @SuppressWarnings("unchecked")
    private void stubBody(String body) throws Exception {
        HttpResponse<String> response = mock(HttpResponse.class);
        when(response.statusCode()).thenReturn(200);
        when(response.body()).thenReturn(body);
        doReturn(response).when(httpClient).send(any(), any());
    }

    /** 取出「這次真的被寫進快取的那個值」，順便釘住 key 的前綴與不得明文帶編號。 */
    private String writtenValue(String certNo, CertificationType type) {
        ArgumentCaptor<String> key = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> val = ArgumentCaptor.forClass(String.class);
        verify(valueOps).set(key.capture(), val.capture(), anyLong(), any());
        assertTrue(key.getValue().startsWith("gov:certv2:" + type.value() + ":"),
                "快取 key 必須用新前綴（舊 gov:cert: 存的是 1/0，結構上分不出查無與已失效），實際="
                        + key.getValue());
        assertNotEquals(certNo, key.getValue(), "編號不得明文入 key");
        return val.getValue();
    }

    /** 3701 單列：可指定回聲編號、狀態與效期。 */
    private static String organicBody(String echoNo, String status, String effectiveDate) {
        return """
                {"RS":"OK","Data":[{
                  "Name":"Acme Road LLC","Address":"臺北市中正區","Tel":"0900000000",
                  "Products":"堅果","ContainCrops":"有機松子","BehaviorType":"B加工",
                  "CompanyName":"慈心有機驗證股份有限公司",
                  "CertOrganicSn":"%s","EffectiveDate":"%s","Status":"%s",
                  "MailingAddress":"臺北市中正區","OldCertOrganicSN":""
                }],"Next":false}
                """.formatted(echoNo, effectiveDate, status);
    }

    private static String tapBody(String echoNo, String validDate, int processDetailItems) {
        StringBuilder process = new StringBuilder();
        for (int i = 0; i < processDetailItems; i++) {
            if (i > 0) {
                process.append(',');
            }
            process.append("""
                    {"ProcessDate":"2025/12/17","ProcessItem":"碾製-%d","ProcessArea":"採後處理場所：某某農會","ProcessMemo":""}"""
                    .formatted(i));
        }
        // TraceCodelist 實測可達 255 KB；這裡塞 60 KB 已足以讓「沒剔除」與「有剔除」差一個數量級。
        String hugeList = "2512170009600018,".repeat(3500);
        return """
                {"RS":"OK","Data":[{
                  "ProductName":"油菜-油菜","OrgID":"100950","Producer":"某某產銷班",
                  "Place":"桃園市八德區","FarmerName":"","PackDate":"2025/12/17",
                  "CertificationName":"中華驗證有限公司","ValidDate":"%s",
                  "StoreInfo":"■ 自營：佳禾農業科技(宜蘭縣壯圍鄉永美路2段83巷1號)",
                  "Tracecode":"%s","LandSecNO":"",
                  "ParentTraceCode":"<a href='https://taft.moa.gov.tw/x'>2512170009600018</a>",
                  "TraceCodelist":"%s","Log_UpdateTime":"2025/12/17",
                  "OperationDetail":[],"ResumeDetail":[],
                  "ProcessDetail":[%s],
                  "CertificateDetail":[]
                }],"Next":false}
                """.formatted(validDate, echoNo, hugeList, process);
    }

    private static String casBody(String echoNo, int rows) {
        StringBuilder sb = new StringBuilder("{\"RS\":\"OK\",\"Data\":[");
        for (int i = 0; i < rows; i++) {
            if (i > 0) {
                sb.append(',');
            }
            sb.append("""
                    {"Emblem_ID":"%s","Factory_CName":"台灣農畜產工業股份有限公司農科廠",
                     "Factory_Address":"屏東縣長治鄉德和村神農東路1號","Factory_Tel":"08-7625088",
                     "Material_Name":"肉品","PType_Name":"中式香腸","Product_Name":"台畜香腸-%d"}"""
                    .formatted(echoNo, i));
        }
        return sb.append("],\"Next\":false}").toString();
    }

    private static final String EMPTY_BODY = "{\"RS\":\"OK\",\"Data\":[],\"Next\":false}";

    // ==================== 防線 1：RS != "OK" ====================

    @Nested
    @DisplayName("防線 1：RS != OK 一律不可用，絕不得落成「查無此認證」")
    class Defense1 {

        /**
         * 實測的災難場景：金鑰失效／配額用盡 → HTTP 200 ＋ {"RS":"ERROR"} ＋ <b>完全沒有 Data 欄</b>。
         *
         * <p>防線拿掉之後：舊 {@code parseResponse} 判「Data 陣列非空」⇒ 回 false
         * ⇒ 全站所有認證編號被判成假的，且沒有任何錯誤訊號。
         */
        @Test
        @DisplayName("金鑰失效（RS=ERROR、無 Data 欄）→ 拋不可用，且不得寫入任何快取")
        void badApiKeyThrowsUnavailable() throws Exception {
            stubBody("{\"RS\":\"ERROR\",\"MSG\":\"API_ID&API_KEY不符,請檢查\"}");

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));

            // 4 參數 set() 同時是快取寫入與 openCircuit 的唯一出口：一次都不該發生。
            verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
        }

        /**
         * 這一支才是真正有鑑別力的那支：{@code RS=ERROR} <b>但帶了空的 Data 陣列</b>。
         *
         * <p>防線 1 一旦被拿掉，這個 body 會安安靜靜地落成 {@code NOT_FOUND} 並被快取一小時
         * ——「上游故障」被寫成了「這張證書不存在」，而且沒有例外、沒有 log、沒有告警。
         */
        @Test
        @DisplayName("RS=ERROR 但帶空 Data：仍必須是不可用，不得塌成 NOT_FOUND")
        void rsErrorWithEmptyDataIsStillUnavailable() throws Exception {
            stubBody("{\"RS\":\"ERROR\",\"MSG\":\"quota exceeded\",\"Data\":[],\"Next\":false}");

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));

            verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
        }

        @Test
        @DisplayName("RS 欄位缺失同樣視為不可用")
        void missingRsIsUnavailable() throws Exception {
            stubBody("{\"Data\":[],\"Next\":false}");

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));
        }

        @Test
        @DisplayName("RS=OK ＋ Data 空陣列才是真正的「查無」")
        void emptyDataIsNotFound() throws Exception {
            stubBody(EMPTY_BODY);

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertFalse(r.found());
            assertEquals(Validity.NOT_FOUND, r.validity());
            assertEquals(0, r.matchedCount());
            assertFalse(r.exists());
        }
    }

    // ==================== 防線 2：編號自我校驗 ====================

    @Nested
    @DisplayName("防線 2：回傳列的編號必須等於送查的編號")
    class Defense2 {

        /**
         * 實測：查詢參數名打錯時，政府 API <b>忽略未知參數並回傳整個資料集</b>（1000 筆）。
         *
         * <p>防線拿掉之後：這個 body（狀態通過、效期未到，但編號是別人的）會回 {@code VALID}
         * ⇒ 在「查得到就自動核准」下，等於<b>自動核准任意號碼</b>。
         */
        @Test
        @DisplayName("回傳列是別人的編號（＝參數名被忽略回傳整個資料集）→ 拋不可用，不得判成有效")
        void echoMismatchThrows() throws Exception {
            stubBody(organicBody("9-999-999999", "通過", FUTURE));

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));

            verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
        }

        @Test
        @DisplayName("回聲欄位整個缺席 → 拋不可用（不得當成「沒有就算過」）")
        void missingEchoFieldThrows() throws Exception {
            stubBody("{\"RS\":\"OK\",\"Data\":[{\"Name\":\"某某農場\",\"Status\":\"通過\"}],\"Next\":false}");

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));
        }

        @Test
        @DisplayName("多列中只要有一列編號不符就拋（不得只查第一列）")
        void anyMismatchedRowThrows() throws Exception {
            stubBody("""
                    {"RS":"OK","Data":[
                      {"CertOrganicSn":"%s","Status":"通過","EffectiveDate":"%s"},
                      {"CertOrganicSn":"9-999-999999","Status":"通過","EffectiveDate":"%s"}
                    ],"Next":false}
                    """.formatted(ORGANIC_NO, FUTURE, FUTURE));

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC));
        }

        @Test
        @DisplayName("三支各自比對自己的回聲欄位（3702 Tracecode / 3703 Emblem_ID）")
        void eachTypeChecksItsOwnEchoField() throws Exception {
            stubBody(tapBody("99999999999999", FUTURE, 1));
            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(TAP_NO, CertificationType.TAP));

            stubBody(casBody("999999", 1));
            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(CAS_NO, CertificationType.CAS));
        }

        @Test
        @DisplayName("編號一致時正常放行（防線不得過度收緊）")
        void matchingEchoPasses() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertTrue(r.exists());
            assertEquals(Validity.VALID, r.validity());
        }
    }

    // ==================== 防線 3：我方截斷 payload ====================

    @Nested
    @DisplayName("防線 3：payload 由我方截斷（$top 實測被伺服器忽略）")
    class Defense3 {

        /**
         * 防線拿掉之後：{@code rawExcerpt} 會原封不動帶著 60 KB 的 {@code TraceCodelist}
         * 進 DB 的 {@code gov_verify_payload} 與 Redis 快取（實測真實值可達 255 KB），
         * 且 {@code ParentTraceCode} 的未跳脫 HTML 會一路送到 ADM 前端。
         */
        @Test
        @DisplayName("3702：剔除 TraceCodelist / StoreInfo / ParentTraceCode，ProcessDetail 截前 5 筆")
        void tapPayloadIsTrimmed() throws Exception {
            String body = tapBody(TAP_NO, FUTURE, 40);
            assertTrue(body.length() > 60_000, "前提：原始 payload 要夠大才驗得出截斷，實際=" + body.length());
            stubBody(body);

            GovVerifyResult r = service.verifyDetailed(TAP_NO, CertificationType.TAP);

            var row = r.rawExcerpt().get(0);
            assertNull(row.get("TraceCodelist"), "TraceCodelist 未被剔除");
            assertNull(row.get("StoreInfo"), "StoreInfo 未被剔除");
            assertNull(row.get("ParentTraceCode"), "ParentTraceCode（含未跳脫 HTML）未被剔除");
            assertEquals(GovCertificationService.MAX_NESTED_ITEMS, row.get("ProcessDetail").size(),
                    "ProcessDetail 未截斷");
            assertTrue(r.rawExcerpt().toString().length() < 4_000,
                    "截斷後仍過大：" + r.rawExcerpt().toString().length());
            // 查驗需要的欄位不能被一起砍掉
            assertEquals("某某產銷班", r.holderName());
            assertEquals("2099-12-31", r.expiresOn());
        }

        /**
         * 實測：一個 {@code Emblem_ID}（工廠級標誌編號）可對到 173 個產品。
         * 防線拿掉之後，那 173 列會整包進快取與 DB payload。
         */
        @Test
        @DisplayName("3703：列數截到上限，但 matchedCount 仍回報截斷前的真實筆數")
        void casRowsAreCappedButCountIsHonest() throws Exception {
            stubBody(casBody(CAS_NO, 173));

            GovVerifyResult r = service.verifyDetailed(CAS_NO, CertificationType.CAS);

            assertEquals(173, r.matchedCount(), "matchedCount 必須是真實筆數，ADM 靠它判斷是否為整廠清單");
            assertEquals(GovCertificationService.MAX_EXCERPT_ROWS, r.rawExcerpt().size(),
                    "rawExcerpt 列數未截斷");
            assertNotNull(r.productSummary());
        }
    }

    // ==================== validity 推導（三支規則不同）====================

    @Nested
    @DisplayName("validity 推導：3701 讀 Status＋效期 / 3702 只有 ValidDate / 3703 found 即 VALID")
    class ValidityDerivation {

        /**
         * 實測 {@code 1-014-123911} 的 {@code Status} 是「結束」，舊版照樣回 {@code exists=true}
         * ——這一支就是那個既有缺陷的迴歸鎖（§8.6）。
         */
        @Test
        @DisplayName("3701 Status=結束（已失效）→ TERMINATED，且 exists=false")
        void organicTerminatedIsNotVerified() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "結束", FUTURE));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertTrue(r.found(), "資料查得到，found 仍是 true");
            assertEquals(Validity.TERMINATED, r.validity());
            assertFalse(r.exists(), "已失效證書不得算 VERIFIED");
            assertFalse(service.verify(ORGANIC_NO, CertificationType.ORGANIC),
                    "boolean 薄 wrapper 必須跟著收窄");
            assertEquals("結束", r.statusRaw());
        }

        @Test
        @DisplayName("3701 Status=終止 → TERMINATED；Status=暫終 → SUSPENDED")
        void organicOtherNegativeStatuses() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "終止", FUTURE));
            assertEquals(Validity.TERMINATED,
                    service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC).validity());

            stubBody(organicBody(ORGANIC_NO, "暫終", FUTURE));
            assertEquals(Validity.SUSPENDED,
                    service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC).validity());
        }

        @Test
        @DisplayName("3701 Status=通過 但效期已過 → EXPIRED（實測 371 筆結束中有 292 筆過期，過期資料照樣回傳）")
        void organicPassedButExpired() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", PAST));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.EXPIRED, r.validity());
            assertFalse(r.exists());
        }

        @Test
        @DisplayName("3701 Status=通過 ＋ 效期未過 → VALID")
        void organicValid() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.VALID, r.validity());
            assertTrue(r.exists());
            assertEquals("慈心有機驗證股份有限公司", r.issuerName());
        }

        @Test
        @DisplayName("3701 狀態值不在已知值域 → UNKNOWN（判讀不出來不等於無效，也不等於有效）")
        void organicUnknownStatus() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "審核中", FUTURE));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.UNKNOWN, r.validity());
            assertFalse(r.exists());
        }

        /**
         * 3702 <b>沒有 Status 欄</b>。若誤把 3701 的規則套上去，這裡會落成 UNKNOWN（找不到 Status），
         * 3702 就永遠不可能 VALID。
         */
        @Test
        @DisplayName("3702 無狀態欄：ValidDate 未過 → VALID，已過 → EXPIRED")
        void tapUsesValidDateOnly() throws Exception {
            stubBody(tapBody(TAP_NO, FUTURE, 1));
            GovVerifyResult ok = service.verifyDetailed(TAP_NO, CertificationType.TAP);
            assertEquals(Validity.VALID, ok.validity());
            assertNull(ok.statusRaw(), "3702 沒有狀態欄，statusRaw 必須是 null 而不是憑空捏一個");

            stubBody(tapBody(TAP_NO, PAST, 1));
            assertEquals(Validity.EXPIRED,
                    service.verifyDetailed(TAP_NO, CertificationType.TAP).validity());
        }

        /**
         * 3703 既無狀態也無效期。若誤套「必須有效期才算 VALID」的規則，CAS 會永遠 UNKNOWN。
         */
        @Test
        @DisplayName("3703 無狀態亦無效期：found 即 VALID，statusRaw / expiresOn 皆為 null")
        void casFoundIsValid() throws Exception {
            stubBody(casBody(CAS_NO, 1));

            GovVerifyResult r = service.verifyDetailed(CAS_NO, CertificationType.CAS);

            assertEquals(Validity.VALID, r.validity());
            assertNull(r.statusRaw());
            assertNull(r.expiresOn());
            assertEquals("台灣農畜產工業股份有限公司農科廠", r.holderName());
        }

        @Test
        @DisplayName("3701 兩列（同證書不同品項）取最有利的一列作為整體結論")
        void organicMultiRowTakesBest() throws Exception {
            stubBody("""
                    {"RS":"OK","Data":[
                      {"CertOrganicSn":"%s","Status":"結束","EffectiveDate":"%s","Products":"其他"},
                      {"CertOrganicSn":"%s","Status":"通過","EffectiveDate":"%s","Products":"堅果"}
                    ],"Next":false}
                    """.formatted(ORGANIC_NO, PAST, ORGANIC_NO, FUTURE));

            GovVerifyResult r = service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.VALID, r.validity());
            assertEquals("通過", r.statusRaw(), "statusRaw 要跟著被選中的那一列");
            assertEquals(2, r.matchedCount());
        }
    }

    // ==================== 效期比較時區（設計書 19 §8.3.9(a)）====================

    /**
     * 政府資料的 {@code EffectiveDate}／{@code ValidDate} 是<b>台灣在地日期</b>（無時區資訊），
     * 所以「今天是幾號」必須用 {@code Asia/Taipei} 問。
     *
     * <p><b>為什麼上面那批 case 證明不了這件事</b>：{@link #FUTURE}／{@link #PAST} 是 2099／2000，
     * 對時區完全不敏感——把正式邏輯改回 UTC，它們照樣全綠。本組刻意把時鐘釘在
     * 「UTC 與台北分屬不同日期」的那一刻，讓時區選擇成為唯一的變因。
     *
     * <p>受測的是<b>正式邏輯選的時區</b>：注入的固定時鐘只提供「什麼時刻」，
     * {@code GovCertificationService} 自己用 {@code GOV_DATA_ZONE} 換算日期
     * ⇒ 把 {@code GOV_DATA_ZONE} 改回 UTC，本組會轉紅。
     */
    @Nested
    @DisplayName("效期比較時區：政府資料是台灣在地日期，必須以 Asia/Taipei 換算「今天」")
    class ExpiryTimeZone {

        /** 台北 2026-08-27 00:30 ＝ UTC 2026-08-26 16:30：這一刻兩個時區分屬不同日期。 */
        private static final Instant TAIPEI_JUST_AFTER_MIDNIGHT = Instant.parse("2026-08-26T16:30:00Z");

        /** 台北 2026-08-26 23:30 ＝ UTC 2026-08-26 15:30：兩個時區同一天。 */
        private static final Instant TAIPEI_JUST_BEFORE_MIDNIGHT = Instant.parse("2026-08-26T15:30:00Z");

        /** 效期落在台北 2026-08-26 這一天。 */
        private static final String EXPIRES_2026_08_26 = "2026/08/26";

        /** 時鐘只提供「時刻」；換算成哪一天是受測邏輯自己的事，所以這裡的 zone 故意給 UTC。 */
        private void freezeAt(Instant instant) {
            ReflectionTestUtils.setField(service, "clock", Clock.fixed(instant, ZoneOffset.UTC));
        }

        /**
         * <b>本組唯一有鑑別力的方向</b>：台北已跨日、UTC 還沒。
         *
         * <p>台北 = 2026-08-27 ⇒ 效期 08-26 已過 ⇒ EXPIRED（正確）。
         * 改用 {@code LocalDate.now(ZoneOffset.UTC)} 則 today = 2026-08-26 ⇒ 判成 VALID，
         * 讓已經過期的證書每天多活 8 小時；在「查得到即自動核准」下等於誤放行。
         */
        @Test
        @DisplayName("3701：台北已跨日、UTC 未跨日 → EXPIRED（用 UTC 會誤判成 VALID）")
        void organicExpiresOnTaipeiCalendarNotUtc() throws Exception {
            freezeAt(TAIPEI_JUST_AFTER_MIDNIGHT);
            stubBody(organicBody(ORGANIC_NO, "通過", EXPIRES_2026_08_26));

            assertEquals(Validity.EXPIRED,
                    service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC).validity(),
                    "台北時間已是 08-27，08-26 到期的證書必須是 EXPIRED；判成 VALID＝仍在用 UTC 曆");
        }

        /** 3702 走同一支 {@code notExpired}，一起鎖住，避免只修 3701 那條路。 */
        @Test
        @DisplayName("3702：台北已跨日、UTC 未跨日 → EXPIRED（ValidDate 走同一支效期比較）")
        void tapExpiresOnTaipeiCalendarNotUtc() throws Exception {
            freezeAt(TAIPEI_JUST_AFTER_MIDNIGHT);
            stubBody(tapBody(TAP_NO, EXPIRES_2026_08_26, 1));

            assertEquals(Validity.EXPIRED,
                    service.verifyDetailed(TAP_NO, CertificationType.TAP).validity(),
                    "3702 只有 ValidDate 可判，時區錯就整個 3702 的到期日錯一天");
        }

        /**
         * 另一邊的界線：<b>效期當日仍算有效</b>。
         *
         * <p>這一支不驗時區（此刻兩時區同一天），驗的是 {@code isBefore} 不得被改成 {@code isAfter} 的否定
         * ——沒有它，「把到期日當天也算過期」這種改法會無聲通過。
         */
        @Test
        @DisplayName("3701：台北到期日當天 23:30 仍是 VALID（效期當日有效，不得提早一天作廢）")
        void organicStillValidOnLastTaipeiDay() throws Exception {
            freezeAt(TAIPEI_JUST_BEFORE_MIDNIGHT);
            stubBody(organicBody(ORGANIC_NO, "通過", EXPIRES_2026_08_26));

            assertEquals(Validity.VALID,
                    service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC).validity(),
                    "台北仍是 08-26，08-26 到期的證書當天仍然有效");
        }
    }

    // ==================== 結構化快取（新前綴 / TTL 分級 / UNKNOWN 不寫）====================

    @Nested
    @DisplayName("結構化快取：新前綴、TTL 分級、UNKNOWN 不寫")
    class StructuredCache {

        @Test
        @DisplayName("VALID → TTL 7 天，且寫的是結構化 JSON 不是 \"1\"")
        void validUsesLongTtl() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            verify(valueOps).set(startsWith("gov:certv2:"), anyString(),
                    eq(props.getCacheTtlExists()), eq(TimeUnit.SECONDS));
            String json = writtenValue(ORGANIC_NO, CertificationType.ORGANIC);
            assertTrue(json.contains("\"validity\":\"VALID\""), "快取內容不是結構化 JSON：" + json);
        }

        @Test
        @DisplayName("NOT_FOUND → TTL 1 小時（打錯字不該被自己的錯誤快取擋住一整天）")
        void notFoundUsesShortTtl() throws Exception {
            stubBody(EMPTY_BODY);

            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            verify(valueOps).set(startsWith("gov:certv2:"), anyString(),
                    eq(props.getCacheTtlNotExists()), eq(TimeUnit.SECONDS));
        }

        @Test
        @DisplayName("EXPIRED / TERMINATED / SUSPENDED → TTL 1 天")
        void invalidStatesUseMediumTtl() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "結束", FUTURE));

            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            verify(valueOps).set(startsWith("gov:certv2:"), anyString(),
                    eq(props.getCacheTtlInvalid()), eq(TimeUnit.SECONDS));
        }

        /**
         * UNKNOWN 是故障不是結論。寫進快取會把一次故障放大成一段時間的錯誤答案——
         * 這一支就是那個放大器的鎖。
         */
        @Test
        @DisplayName("UNKNOWN → 完全不寫快取")
        void unknownIsNeverCached() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "審核中", FUTURE));

            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

            verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
        }

        @Test
        @DisplayName("寫進去的 JSON 讀回來要還原成同一個結論（背景寫、同步讀，跨路徑的唯一介面）")
        void cacheRoundTrip() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "結束", FUTURE));
            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);
            String json = writtenValue(ORGANIC_NO, CertificationType.ORGANIC);

            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn(json);

            GovVerifyResult back = service.readCachedResult(ORGANIC_NO, CertificationType.ORGANIC);

            assertNotNull(back);
            assertEquals(Validity.TERMINATED, back.validity());
            assertTrue(back.found());
            assertFalse(back.exists());
            assertEquals("結束", back.statusRaw());
            assertEquals(ORGANIC_NO, back.certNo());
        }

        @Test
        @DisplayName("readCachedResult 一律不發外部呼叫（§8.2 的結構保證）")
        void readCachedResultNeverCallsGovApi() throws Exception {
            when(valueOps.get(anyString())).thenReturn(null);

            assertNull(service.readCachedResult(ORGANIC_NO, CertificationType.ORGANIC));

            verify(httpClient, never()).send(any(), any());
        }

        @Test
        @DisplayName("不支援線上查驗的類型（3704/3705/3706）→ 直接回 null，不查 Redis 也不打 API")
        void unsupportedTypesShortCircuit() throws Exception {
            assertNull(service.readCachedResult("HACCP-2026-DP-0047", CertificationType.HACCP));

            verify(valueOps, never()).get(anyString());
            verify(httpClient, never()).send(any(), any());
        }

        @Test
        @DisplayName("快取內容壞掉 → 視為未命中，不得讓壞快取變成查驗結論")
        void corruptCacheIsTreatedAsMiss() {
            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn("1");

            assertNull(service.readCachedResult(ORGANIC_NO, CertificationType.ORGANIC));
        }
    }

    // ==================== 背景查驗路徑 ====================

    @Nested
    @DisplayName("背景查驗：90 秒 timeout，且與同步 circuit breaker 完全解耦")
    class BackgroundPath {

        /**
         * §8.3 的核心：同步階段一旦開了 circuit（TTL 60s、Redis 全域共享），
         * 背景查驗若也讀同一把鎖，就會在 60 秒窗口內被短路成失敗——整套背景補課機制被封死。
         */
        @Test
        @DisplayName("circuit 開著時：同步路徑短路，背景路徑照打不誤")
        void backgroundBypassesOpenCircuit() throws Exception {
            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn(null);
            when(valueOps.get(startsWith("gov:cert:circuit:"))).thenReturn("1");
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC),
                    "同步路徑仍必須尊重 circuit");

            GovVerifyResult r = service.verifyDetailedForBackground(ORGANIC_NO, CertificationType.ORGANIC);
            assertEquals(Validity.VALID, r.validity());
        }

        /**
         * 反向也要成立：3702 實測 33–82 秒，背景逾時是常態。
         * 若背景失敗會寫共享 circuit，等於讓常態性的慢把同步 advisory 端點打掛 60 秒。
         */
        @Test
        @DisplayName("背景查驗失敗不得打開同步 circuit")
        void backgroundFailureDoesNotOpenCircuit() throws Exception {
            doThrow(new java.net.http.HttpTimeoutException("request timed out"))
                    .when(httpClient).send(any(), any());

            assertThrows(GovCertificationService.GovApiUnavailableException.class,
                    () -> service.verifyDetailedForBackground(ORGANIC_NO, CertificationType.ORGANIC));

            verify(valueOps, never()).set(anyString(), anyString(), anyLong(), any());
        }

        @Test
        @DisplayName("背景用 90 秒 read-timeout，同步維持 10 秒")
        void timeoutsDiffer() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
            ArgumentCaptor<HttpRequest> req = ArgumentCaptor.forClass(HttpRequest.class);

            service.verifyDetailedForBackground(ORGANIC_NO, CertificationType.ORGANIC);
            verify(httpClient).send(req.capture(), any());
            assertEquals(Duration.ofMillis(props.getBackgroundReadTimeout()),
                    req.getValue().timeout().orElseThrow(),
                    "背景 timeout 設 70 秒會讓實測 11 筆樣本中至少 4 筆逾時（§8.7）");
            assertEquals(Duration.ofMillis(90_000), req.getValue().timeout().orElseThrow());
        }

        @Test
        @DisplayName("同步路徑仍是 10 秒（使用者在等畫面，不能讓他等 90 秒）")
        void syncTimeoutUnchanged() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
            ArgumentCaptor<HttpRequest> req = ArgumentCaptor.forClass(HttpRequest.class);

            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);
            verify(httpClient).send(req.capture(), any());
            assertEquals(Duration.ofMillis(props.getReadTimeout()),
                    req.getValue().timeout().orElseThrow());
        }
    }

    // ==================== ADM 手動重新查驗：繞過結果快取 ====================

    /**
     * §9.3。dev 實測的病徵：對已查驗過的申請按「重新查驗」，{@code version} 有 bump、
     * {@code updated_at} 有動，但 {@code gov_verified_at} 與 payload 的 {@code queriedAt}
     * <b>完全沒變</b>——因為它走了會讀結果快取的路徑，而 {@code VALID} 的 TTL 是 <b>7 天</b>。
     *
     * <p>後果：這顆按鈕存在的第一個理由（「證書可能在申請之後才登記上政府資料庫」）
     * 直接失效——賣家補登完，ADM 七天內按幾次都拿到同一個舊答案，而<b>畫面上沒有任何線索</b>。
     */
    @Nested
    @DisplayName("§9.3 ADM 重新查驗：一定真的打政府 API")
    class ForceRefreshPath {

        /**
         * 🔴 核心不變式。斷言的是「<b>那次 get 從來沒有發出去</b>」而不是「結果不等於快取值」：
         * 後者在快取剛好與新答案相同時是恆真的，等於什麼都沒守。
         *
         * <p>把 {@code doVerify} 的 {@code if (!bypassResultCache)} 拿掉，這支立刻轉紅。
         */
        @Test
        @DisplayName("🔴 快取有值也照打政府 API，而且連讀都不讀那把 key")
        void forceRefreshNeverReadsResultCache() throws Exception {
            // 快取裡躺著一筆「有效」的舊結果（就是那個最長活七天的東西）
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);
            String staleJson = writtenValue(ORGANIC_NO, CertificationType.ORGANIC);
            org.mockito.Mockito.clearInvocations(valueOps, httpClient);

            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn(staleJson);
            // 政府端這次的答案已經不一樣了（證書被終止）——只有真的打，才看得到這件事
            stubBody(organicBody(ORGANIC_NO, "結束", FUTURE));

            GovVerifyResult r = service.verifyDetailedForceRefresh(ORGANIC_NO, CertificationType.ORGANIC);

            verify(httpClient).send(any(), any());
            verify(valueOps, never()).get(startsWith("gov:certv2:"));
            assertEquals(Validity.TERMINATED, r.validity(),
                    "拿到的仍是快取裡那筆 VALID ⇒ 繞過失效");
        }

        /**
         * 對照組，用來證明上面那支有鑑別力：同一份快取、同一段 stub，
         * 走背景路徑就<b>會</b>命中快取並回舊答案。
         *
         * <p>沒有這一支的話，「forceRefresh 打了 API」可能只是因為快取 stub 根本沒生效。
         */
        @Test
        @DisplayName("對照組：同一份快取，背景路徑照樣命中並回舊答案（證明 stub 真的有效）")
        void backgroundPathStillHitsTheSameCache() throws Exception {
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
            service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);
            String staleJson = writtenValue(ORGANIC_NO, CertificationType.ORGANIC);
            org.mockito.Mockito.clearInvocations(valueOps, httpClient);

            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn(staleJson);
            stubBody(organicBody(ORGANIC_NO, "結束", FUTURE));

            GovVerifyResult r = service.verifyDetailedForBackground(ORGANIC_NO, CertificationType.ORGANIC);

            verify(httpClient, never()).send(any(), any());
            assertEquals(Validity.VALID, r.validity());
        }

        /**
         * 繞過的是<b>讀</b>，不是寫。拿到新答案卻不回寫，同步路徑
         * （{@code readCachedResult}，商品建立／匯入 commit 用）會繼續吃那筆最長七天的舊值
         * ——ADM 已經知道答案變了，其他路徑卻不知道。回寫同時把 TTL 從現在重新起算。
         */
        @Test
        @DisplayName("仍然回寫快取（繞的是讀不是寫），否則同步路徑會繼續吃七天前的舊值")
        void forceRefreshStillWritesCache() throws Exception {
            when(valueOps.get(startsWith("gov:certv2:"))).thenReturn(null);
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            service.verifyDetailedForceRefresh(ORGANIC_NO, CertificationType.ORGANIC);

            ArgumentCaptor<String> key = ArgumentCaptor.forClass(String.class);
            verify(valueOps).set(key.capture(), anyString(),
                    eq(TimeUnit.DAYS.toSeconds(7)), eq(TimeUnit.SECONDS));
            assertTrue(key.getValue().startsWith("gov:certv2:"), "實際=" + key.getValue());
        }

        /**
         * 沿用背景路徑的其餘性質：90 秒 read-timeout（3702 實測 33–82 秒），
         * 且不讀不寫同步 circuit——ADM 按下去的那一刻不該被同步路徑近期的失敗擋掉。
         */
        @Test
        @DisplayName("沿用背景路徑的 90 秒 timeout，且 circuit 開著也照打")
        void forceRefreshKeepsBackgroundTimeoutAndBypassesCircuit() throws Exception {
            when(valueOps.get(startsWith("gov:cert:circuit:"))).thenReturn("1");
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
            ArgumentCaptor<HttpRequest> req = ArgumentCaptor.forClass(HttpRequest.class);

            GovVerifyResult r = service.verifyDetailedForceRefresh(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.VALID, r.validity());
            verify(httpClient).send(req.capture(), any());
            assertEquals(Duration.ofMillis(90_000), req.getValue().timeout().orElseThrow());
        }

        /**
         * Redis 整個失聯時也不得變成「查驗失敗」：繞過本來就不需要 Redis，
         * 而寫入失敗只是少一次快取（02 §2.7.3）。
         */
        @Test
        @DisplayName("Redis 失聯不影響：本來就不讀，寫不進去也只是少一次快取")
        void redisOutageDoesNotBreakForceRefresh() throws Exception {
            doThrow(new org.springframework.dao.QueryTimeoutException("redis down"))
                    .when(valueOps).set(anyString(), anyString(), anyLong(), any());
            stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

            GovVerifyResult r = service.verifyDetailedForceRefresh(ORGANIC_NO, CertificationType.ORGANIC);

            assertEquals(Validity.VALID, r.validity());
        }
    }

    // ==================== verify() 薄 wrapper 與新前綴共用 ====================

    @Test
    @DisplayName("verify() 走的是 verifyDetailed 的內部實作：讀寫同一個新前綴")
    void legacyVerifySharesNewPrefix() throws Exception {
        stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));

        assertTrue(service.verify(ORGANIC_NO, CertificationType.ORGANIC));

        ArgumentCaptor<String> key = ArgumentCaptor.forClass(String.class);
        verify(valueOps).set(key.capture(), anyString(), anyLong(), any());
        assertTrue(key.getValue().startsWith("gov:certv2:"),
                "advisory／同步路徑若還寫舊前綴，背景查驗回寫的結果永遠餵不到同步路徑，實際=" + key.getValue());
    }

    @Test
    @DisplayName("查詢參數名與防線 2 的回聲欄位同源：參數名打錯時校驗欄位也會跟著錯，防線就白做了")
    void queryParamAndEchoFieldShareOneSource() throws Exception {
        stubBody(organicBody(ORGANIC_NO, "通過", FUTURE));
        ArgumentCaptor<HttpRequest> req = ArgumentCaptor.forClass(HttpRequest.class);

        service.verifyDetailed(ORGANIC_NO, CertificationType.ORGANIC);

        verify(httpClient).send(req.capture(), any());
        assertTrue(req.getValue().uri().getQuery().contains("CertOrganicSn=" + ORGANIC_NO),
                "實際 query=" + req.getValue().uri().getQuery());
    }
}
