package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.product.config.GovApiProperties;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.net.ConnectException;
import java.net.URI;
import java.net.UnknownHostException;
import java.net.http.HttpClient;
import java.net.http.HttpConnectTimeoutException;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.regex.Pattern;

/**
 * 農業部開放資料 API 認證驗證服務（設計書 19 §8）。
 *
 * <p><b>本類別的對外形狀</b>：
 * <ul>
 *   <li>{@link #verifyDetailed} — 同步路徑（advisory 端點）用，read-timeout 10 秒、走 circuit breaker。</li>
 *   <li>{@link #verifyDetailedForBackground} — 背景查驗 consumer 用，read-timeout 90 秒、
 *       <b>繞過 circuit breaker</b>（§8.3）。</li>
 *   <li>{@link #verifyDetailedForceRefresh} — ADM 手動「重新查驗」用，同背景路徑再加
 *       <b>繞過結果快取</b>（§9.3）：一定會真的打政府 API。</li>
 *   <li>{@link #verify} — 既有 boolean 介面，<b>薄 wrapper</b>，內部走 {@link #verifyDetailed}，
 *       因此與背景查驗讀寫<b>同一個快取前綴</b>（§8.3.1）。</li>
 *   <li>{@link #readCachedResult} — 只讀結構化快取、<b>一律不發外部呼叫</b>（§8.2 同步階段用）。</li>
 * </ul>
 *
 * <p><b>三道防線（§8.4，全部是實測發現的既有災難級缺陷）</b>：
 * <ol>
 *   <li>{@code RS != "OK"} → 拋 {@link GovApiUnavailableException}。金鑰失效／配額用盡時政府 API
 *       回 <b>HTTP 200 且完全沒有 {@code Data} 欄</b>，舊 {@code parseResponse} 判「陣列非空」
 *       ⇒ 回 false ＝「認證不存在」⇒ 金鑰一出事全站認證靜默變成假的。</li>
 *   <li>編號自我校驗。實測送未知參數名時伺服器<b>忽略該參數並回傳整個資料集</b>（1000 筆）
 *       ⇒ 參數名一旦打錯，任何編號都會「驗證通過」。</li>
 *   <li>我方截斷 payload。{@code $top} 實測<b>被伺服器忽略</b>，不能靠它限制大小；
 *       3702 的 {@code TraceCodelist} 實測可達數百 KB，3703 一個編號可對到 173 列。</li>
 * </ol>
 */
@Service
public class GovCertificationService {

    private static final ObsLogger log = ObsLogger.forClass(GovCertificationService.class);

    /**
     * 結構化查驗快取前綴（§8.3.1）。
     *
     * <p>刻意與舊的 {@code gov:cert:}（只存 {@code "1"/"0"}）分家：舊格式結構上無法區分
     * 「查無」與「已失效」，而 §8.2 要求同步命中時要能分別落 {@code NOT_FOUND} 與 {@code EXPIRED}。
     * 舊 key 不再讀寫、自然過期。
     */
    private static final String CACHE_PREFIX = "gov:certv2:";

    private static final String CIRCUIT_PREFIX = "gov:cert:circuit:";
    private static final long CIRCUIT_TTL_SECONDS = 60;

    /** 防線 3：{@code rawExcerpt} 保留的最大列數（3703 實測一個編號可對到 173 列）。 */
    static final int MAX_EXCERPT_ROWS = 20;

    /** 防線 3：3702 巢狀陣列保留的最大元素數（{@code ProcessDetail} 實測可達 112 筆）。 */
    static final int MAX_NESTED_ITEMS = 5;

    /** {@code productSummary} 聚合的最大品項數與總長度。 */
    private static final int MAX_PRODUCT_SUMMARY_ITEMS = 20;
    private static final int MAX_PRODUCT_SUMMARY_LENGTH = 500;

    /**
     * 防線 3：3702 直接剔除的超大／無查驗價值欄位。
     *
     * <p>{@code TraceCodelist} 實測可達 255 KB；{@code ParentTraceCode} 內含<b>未跳脫 HTML</b>
     * （{@code <a href="…">}），留著只會把 XSS 風險往 ADM 推。
     */
    private static final Set<String> TAP_DROP_FIELDS =
            Set.of("TraceCodelist", "StoreInfo", "ParentTraceCode");

    /** 防線 3：3702 的四個巢狀陣列，一律截前 {@link #MAX_NESTED_ITEMS} 筆。 */
    private static final Set<String> TAP_NESTED_ARRAYS =
            Set.of("OperationDetail", "ResumeDetail", "ProcessDetail", "CertificateDetail");

    /** 政府 API 的日期格式（實測 {@code 2028/10/14}）；另接受 ISO {@code 2028-10-14}。 */
    private static final DateTimeFormatter GOV_DATE = DateTimeFormatter.ofPattern("yyyy/MM/dd");

    /**
     * 政府開放資料的日期時區（設計書 19 §8.3.9(a)）。
     *
     * <p>{@code EffectiveDate}／{@code ValidDate} 是<b>台灣在地日期</b>（{@code yyyy/MM/dd}，
     * 不帶任何時區資訊）⇒ 「今天是幾號」必須用台北時區問，否則整段 UTC 16:00–24:00
     * （＝台北隔日 00:00–08:00）我方看到的日期會比證書所依據的日曆<b>晚一天</b>，
     * 讓已經過期的證書多活 8 小時（在「查得到即自動核准」下等於誤放行）。
     *
     * <p>⚠️ 與「時間一律 UTC」不衝突：那條管的是<b>儲存與傳輸</b>——本類別寫進
     * {@code queriedAt} 與快取的時間仍一律 UTC（見 {@link #nowIso()}）；這裡只用於
     * 與外部在地日期做語意比較。
     */
    private static final ZoneId GOV_DATA_ZONE = DateTimeUtils.TAIWAN_ZONE;

    /**
     * 認證編號長度上限。
     *
     * <p>certifications 存成 JSONB（V1__init_product_schema.sql），沒有可比照的欄位長度；
     * repo 內最長的實際樣本是 18 碼（seed data 的 {@code HACCP-2026-DP-0047}），可線上驗證的
     * 三型最長 16 碼，取 64 留約 3.5 倍餘裕，同時讓送往政府 API 的查詢字串長度有界。
     */
    public static final int CERT_NO_MAX_LENGTH = 64;

    /**
     * 認證編號合法字元：英數 + 連字號 + 底線。
     *
     * <p>農業部三支端點沒有公開的編號格式規範，{@code docs/開發規範/} 也沒有記載，因此取
     * 保守通用集合。實際樣本（設計書 19 §3/§7.1 的 {@code 1-009-300022}、seed data 的
     * {@code 1-008-131005}／{@code TAP-2025-0032}／{@code CAS-2025-1234}）都落在這個集合內。
     *
     * <p>這一組字元全屬 RFC 3986 的 unreserved character，因此 {@code buildGovApiUrl} 的
     * 原字串串接不會產生需要 percent-encoding 的字元，{@code URI.create} 也不會因此失敗。
     */
    private static final Pattern CERT_NO_PATTERN =
            Pattern.compile("^[A-Za-z0-9_-]{1," + CERT_NO_MAX_LENGTH + "}$");

    private final GovApiProperties govApiProperties;
    private final StringRedisTemplate redisTemplate;
    private final HttpClient httpClient;
    private final ObjectMapper objectMapper;

    /**
     * 效期比較的時間來源。
     *
     * <p>只取 {@link Clock#instant()}，<b>時區一律由 {@link #GOV_DATA_ZONE} 決定</b>
     * ——這樣測試注入固定時鐘時控制的是「什麼時刻」，而「用哪個時區換算日期」仍是受測的正式邏輯。
     * 測試以 {@code ReflectionTestUtils.setField(service, "clock", …)} 注入（與 {@code httpClient} 同慣例）。
     */
    private final Clock clock;

    public GovCertificationService(GovApiProperties govApiProperties,
                                   StringRedisTemplate redisTemplate) {
        this.govApiProperties = govApiProperties;
        this.redisTemplate = redisTemplate;
        this.objectMapper = new ObjectMapper();
        this.clock = Clock.systemUTC();
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofMillis(govApiProperties.getConnectTimeout()))
                .build();
    }

    // ==================== 對外型別 ====================

    /**
     * 查驗結論（§8.5 推導規則三支各不相同）。
     *
     * <p>{@code UNKNOWN} 是「我方或上游故障／回應無法判讀」，<b>不是</b>查驗結論
     * ⇒ 一律不寫快取（把一次故障快取起來，等於把它放大成一段時間的錯誤答案）。
     */
    public enum Validity {
        VALID, EXPIRED, TERMINATED, SUSPENDED, NOT_FOUND, UNKNOWN
    }

    /**
     * 結構化查驗結果（§8.3.1）。
     *
     * @param certType   認證類型 value（{@code 3701}／{@code 3702}／{@code 3703}）
     * @param certNo     送查的編號（＝防線 2 的比對基準）
     * @param queriedAt  查驗時間（ISO 8601 UTC）
     * @param found      政府 API 是否查得到這個編號
     * @param validity   結論，見 {@link Validity}
     * @param statusRaw  原始狀態字串，<b>只有 3701 有</b>（通過／結束／終止／暫終）
     * @param expiresOn  效期，3703 沒有；可解析時正規化為 {@code yyyy-MM-dd}，否則原樣保留
     * @param holderName 業者名（3701 {@code Name}／3702 {@code Producer}／3703 {@code Factory_CName}）
     * @param holderAddress 地址
     * @param issuerName 驗證機構（3703 沒有）
     * @param productSummary 品項摘要，跨列去重後聚合
     * @param matchedCount 命中列數（<b>截斷前</b>的真實筆數，供 ADM 判斷是否為整廠清單）
     * @param rawExcerpt 修剪後的原始列（防線 3）
     */
    public record GovVerifyResult(
            String certType,
            String certNo,
            String queriedAt,
            boolean found,
            Validity validity,
            String statusRaw,
            String expiresOn,
            String holderName,
            String holderAddress,
            String issuerName,
            String productSummary,
            int matchedCount,
            JsonNode rawExcerpt
    ) {
        /** 舊 boolean 契約的等價式：{@code exists = found && validity == VALID}（§9.4）。 */
        public boolean exists() {
            return found && validity == Validity.VALID;
        }
    }

    // ==================== 入口 ====================

    /**
     * 認證編號字元白名單檢查。
     *
     * <p>Gateway（{@code product.go}）與 gRPC 入口（{@code ProductGrpcService.verifyCertification}）
     * 都要先過這一關；兩端規則必須一致，改動時兩邊一起改。
     *
     * @param certNo 認證編號
     * @return true=合法
     */
    public static boolean isValidCertNo(String certNo) {
        return certNo != null && CERT_NO_PATTERN.matcher(certNo).matches();
    }

    /** 該類型是否支援線上查驗（3704／3705／3706 不支援，§8.5）。 */
    public static boolean isOnlineVerifiable(CertificationType certType) {
        return certType == CertificationType.ORGANIC
                || certType == CertificationType.TAP
                || certType == CertificationType.CAS;
    }

    /**
     * 驗證認證編號是否「存在且有效」。
     *
     * <p><b>薄 wrapper</b>：內部走 {@link #verifyDetailed}，與背景查驗共用同一個快取前綴。
     * 語意較舊版收窄——舊版只判資料陣列非空，已失效證書（{@code Status:"結束"}）照樣回 true（§8.6）。
     *
     * @param certNo 認證編號
     * @param certType 認證類型
     * @return true=查得到且目前有效
     */
    public boolean verify(String certNo, CertificationType certType) {
        return verifyDetailed(certNo, certType).exists();
    }

    /**
     * 同步路徑查驗（advisory 端點）：read-timeout 10 秒、走 circuit breaker。
     *
     * <p>⚠️ 不得在商品寫入鏈（IMPORT_COMMIT／PUBLISH）上呼叫本方法：逾時會打開 per-type、
     * TTL 60 秒、Redis 全域共享的 circuit，把緊接著排出去的背景查驗一起封死（§6.5.3）。
     */
    public GovVerifyResult verifyDetailed(String certNo, CertificationType certType) {
        return doVerify(certNo, certType, false, false);
    }

    /**
     * 背景查驗（MQ consumer）：read-timeout 90 秒、<b>繞過 circuit breaker</b>。
     *
     * <p>90 秒的依據見 §8.7：3702 實測 11 次取樣落在 33–82 秒，設 70 秒會讓多數樣本逾時，
     * 「背景補課」機制形同虛設。
     *
     * <p>繞過 circuit 的理由（§8.3）：背景路徑本來就預期慢，不該被同步路徑的判斷影響；
     * 反向也成立——背景的慢不該回頭把同步 circuit 打開。因此本路徑<b>既不讀也不寫</b> circuit key。
     * 可靠性由 MQ 的 retry／DLQ 提供。
     */
    public GovVerifyResult verifyDetailedForBackground(String certNo, CertificationType certType) {
        return doVerify(certNo, certType, true, false);
    }

    /**
     * ADM 手動「重新查驗」（§9.3）：背景路徑的 timeout 與 circuit 行為，外加<b>繞過結果快取</b>。
     *
     * <p>🔴 <b>為什麼非繞不可</b>：結果快取的 TTL 對 {@code VALID} 是 <b>7 天</b>（§8.3.1）。
     * 而這顆按鈕存在的第一個理由就是「證書可能在申請<b>之後</b>才登記上政府資料庫」——
     * 讀快取的話，賣家補登完證書，ADM 七天內按幾次都拿到同一個舊答案，
     * 而 {@code version} 照樣 bump、{@code updated_at} 照樣動，
     * <b>畫面上沒有任何線索說這是快取</b>。等於這顆按鈕在最需要它的情境下靜默失效。
     *
     * <p>⚠️ <b>代價是真的消耗政府 API 每日配額</b>，所以只有 ADM 手動觸發走這條路；
     * 背景查驗（MQ consumer）與同步路徑仍照常讀快取。ADM 端已在按鈕旁常駐說明
     * （{@code hawkr-admin} 的 {@code REVERIFY_HINT}）。
     *
     * <p>✅ <b>仍然寫回快取</b>：拿到新答案卻不回寫，同步路徑（{@link #readCachedResult}）
     * 會繼續吃那筆最長七天的舊值——ADM 已經知道答案變了，其他路徑卻不知道。
     * 回寫同時把 TTL 從現在重新起算。
     */
    public GovVerifyResult verifyDetailedForceRefresh(String certNo, CertificationType certType) {
        return doVerify(certNo, certType, true, true);
    }

    /**
     * 只讀結構化快取，<b>一律不發外部呼叫</b>（§8.2）。
     *
     * <p>同步階段（商品建立／更新／匯入 commit）唯一允許使用的查驗入口：零外部呼叫
     * ⇒ 零延遲、零 circuit breaker 副作用，商品建立永不被政府 API 影響。
     *
     * @return 快取命中的結構化結果；未命中／Redis 失聯／不支援線上查驗 → {@code null}
     */
    public GovVerifyResult readCachedResult(String certNo, CertificationType certType) {
        if (certType == null || !isOnlineVerifiable(certType) || !isValidCertNo(certNo)) {
            return null;
        }
        try {
            String cached = redisTemplate.opsForValue().get(buildCacheKey(certType, certNo));
            return cached == null ? null : fromCacheJson(cached);
        } catch (DataAccessException e) {
            log.warn("cacheUnavailable", "Redis 失聯，認證查驗快取讀取略過: " + e.getMessage());
            return null;
        }
    }

    // ==================== 主流程 ====================

    /**
     * @param background          背景路徑：90 秒 read-timeout，且既不讀也不寫 circuit breaker
     * @param bypassResultCache   跳過結果快取的<b>讀取</b>（寫入照舊，見 {@link #verifyDetailedForceRefresh}）
     */
    private GovVerifyResult doVerify(String certNo, CertificationType certType,
                                     boolean background, boolean bypassResultCache) {
        // 0. 字元白名單。這一關同時是安全邊界：certNo 之後會原字串接進政府 API 的查詢字串，
        //    而該 URL 尾端帶 api_key。放行 URI 保留字元等同讓呼叫端隨手觸發 URI.create 失敗，
        //    把整條含金鑰的 URL 推進例外訊息。擋在 try 之前也避免爛輸入誤觸 circuit breaker。
        if (!isValidCertNo(certNo)) {
            throw new IllegalArgumentException("認證編號格式不正確");
        }

        String cacheKey = buildCacheKey(certType, certNo);
        String circuitKey = CIRCUIT_PREFIX + certType.code();

        // 1+2. 查 Redis 結果快取 + circuit breaker
        //   Redis 失聯時降級（02 §2.7.3）：略過快取與 circuit，直接呼叫政府 API。
        try {
            // bypassResultCache（ADM 手動重新查驗，§9.3）：連 get 都不發。
            // 🔴 這裡的判斷不能寫成「讀了但不用」——那樣一旦有人把 return 補回來就沒有任何
            //    東西會轉紅；「沒有發出那次 get」本身才是測得到的不變式。
            if (!bypassResultCache) {
                String cached = redisTemplate.opsForValue().get(cacheKey);
                if (cached != null) {
                    GovVerifyResult hit = fromCacheJson(cached);
                    if (hit != null) {
                        log.info("cacheHit", "認證查驗快取命中: type=" + certType.value()
                                + ", certNo=" + certNo + ", validity=" + hit.validity());
                        return hit;
                    }
                }
            }

            // circuit breaker（上游近期失敗 → 直接快回，避免每次卡 timeout）。
            // 背景路徑刻意不讀：見 verifyDetailedForBackground 的說明。
            if (!background) {
                String circuit = redisTemplate.opsForValue().get(circuitKey);
                if (circuit != null) {
                    log.warn("circuitOpen", "上游政府 API circuit breaker 短路命中: type=" + certType.value());
                    throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
                }
            }
        } catch (DataAccessException e) {
            log.warn("cacheUnavailable", "Redis 失聯，認證查驗略過快取/circuit 直打政府 API: " + e.getMessage());
        }

        // 3. 呼叫政府 API
        String url = buildGovApiUrl(certType, certNo);
        long readTimeout = background
                ? govApiProperties.getBackgroundReadTimeout()
                : govApiProperties.getReadTimeout();
        log.info("apiCall", "呼叫政府 API: type=" + certType.value() + ", certNo=" + certNo
                + ", background=" + background + ", forceRefresh=" + bypassResultCache
                + ", readTimeoutMs=" + readTimeout);

        try {
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(url))
                    .timeout(Duration.ofMillis(readTimeout))
                    .GET()
                    .build();

            HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());

            int status = response.statusCode();
            if (status != 200) {
                log.error("apiError", "政府 API 回應異常: type=" + certType.value() + ", status=" + status);
                openCircuit(circuitKey, background);
                throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
            }

            // 4. 解析回應（防線 1 / 2 / 3 都在裡面）
            GovVerifyResult result = parseDetailed(certType, certNo, response.body());

            // 5. 寫入快取（Redis 失聯時 log.warn 略過，不得讓快取寫入失敗誤判為政府 API 失敗，02 §2.7.3）
            writeCache(cacheKey, result);

            log.info("result", "認證查驗結果: type=" + certType.value() + ", certNo=" + certNo
                    + ", found=" + result.found() + ", validity=" + result.validity()
                    + ", matched=" + result.matchedCount());
            return result;

        } catch (GovApiUnavailableException | GovApiResponseException e) {
            throw e;
        } catch (HttpConnectTimeoutException | ConnectException | UnknownHostException e) {
            log.error("connectFail", "政府 API 連線失敗: type=" + certType.value() + ", certNo=" + certNo + ", cause=" + e.getClass().getSimpleName());
            openCircuit(circuitKey, background);
            throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
        } catch (java.net.http.HttpTimeoutException e) {
            log.error("timeout", "政府 API 讀取超時: type=" + certType.value() + ", certNo=" + certNo);
            openCircuit(circuitKey, background);
            throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
        } catch (Exception e) {
            // 只記例外類別名稱，比照上面兩個分支：例外訊息可能內嵌整條含 api_key 的 URL
            // （例如 URI.create 的 IllegalArgumentException），整個物件交給 logger 就是外洩。
            log.error("error", "政府 API 呼叫失敗: type=" + certType.value() + ", certNo=" + certNo + ", cause=" + e.getClass().getSimpleName());
            openCircuit(circuitKey, background);
            throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
        }
    }

    /**
     * 觸發 circuit breaker：60 秒內後續同步請求直接快回，避免每次卡 timeout。
     *
     * <p>背景路徑一律略過——它的逾時是常態（3702 實測 33–82 秒），寫進共享 circuit
     * 會把同步 advisory 端點一起打掛。
     */
    private void openCircuit(String circuitKey, boolean background) {
        if (background) {
            return;
        }
        try {
            redisTemplate.opsForValue().set(circuitKey, "1", CIRCUIT_TTL_SECONDS, TimeUnit.SECONDS);
        } catch (Exception ex) {
            log.warn("circuitSetFail", "circuit breaker 寫入失敗: " + ex.getMessage());
        }
    }

    // ==================== 解析與三道防線 ====================

    /**
     * 解析政府 API 回應。
     * 格式：{@code { "RS": "OK", "Data": [...], "Next": false }}
     */
    private GovVerifyResult parseDetailed(CertificationType certType, String certNo, String body) {
        JsonNode root;
        try {
            root = objectMapper.readTree(body);
        } catch (Exception e) {
            log.error("parseError", "政府 API 回應格式異常: type=" + certType.value(), e);
            throw new GovApiResponseException(GrpcResponseCode.GOV_CERT_API_RESPONSE_INVALID.message());
        }

        // ---- 防線 1：RS != "OK" 一律視為不可用，絕不得落成「查無此認證」 ----
        // 實測：金鑰錯／失效／配額用盡 → {"RS":"ERROR","MSG":"API_ID&API_KEY不符,請檢查"}，
        // HTTP 200 且完全沒有 Data 欄。舊版判「陣列非空」⇒ 回 false ⇒ 全站認證靜默變成假的。
        JsonNode rs = root == null ? null : root.get("RS");
        if (rs == null || !"OK".equals(rs.asText())) {
            // 只記 RS 值，不記 MSG：MSG 由上游控制，不保證不回聲請求內容。
            log.error("govApiNotOk", "政府 API 回報非 OK，視為不可用（不得判為查無）: type="
                    + certType.value() + ", rs=" + (rs == null ? "<missing>" : rs.asText()));
            throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
        }

        JsonNode data = root.get("Data");
        if (data == null || !data.isArray()) {
            log.error("parseError", "政府 API 回應缺少 Data 陣列: type=" + certType.value());
            throw new GovApiResponseException(GrpcResponseCode.GOV_CERT_API_RESPONSE_INVALID.message());
        }

        int matchedCount = data.size();
        if (matchedCount == 0) {
            return new GovVerifyResult(certType.value(), certNo, nowIso(), false, Validity.NOT_FOUND,
                    null, null, null, null, null, null, 0, objectMapper.createArrayNode());
        }

        // ---- 防線 2：編號自我校驗 ----
        // 實測：送未知參數名時伺服器忽略該參數並回傳整個資料集（1000 筆）
        // ⇒ 參數名一旦打錯，任何編號都會「驗證通過」＝自動核准任意號碼。
        String echoField = echoFieldOf(certType);
        for (JsonNode row : data) {
            JsonNode echo = row == null ? null : row.get(echoField);
            String echoValue = echo == null ? null : echo.asText("");
            if (echoValue == null || !certNo.trim().equalsIgnoreCase(echoValue.trim())) {
                log.error("echoMismatch", "政府 API 回傳列的編號與送查編號不符，視為不可用: type="
                        + certType.value() + ", field=" + echoField + ", certNo=" + certNo
                        + ", matched=" + matchedCount);
                throw new GovApiUnavailableException(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.message());
            }
        }

        // ---- 結論推導（§8.5，三支規則不同）＋ 防線 3（截斷）----
        JsonNode best = null;
        Validity bestValidity = null;
        for (JsonNode row : data) {
            Validity v = deriveRowValidity(certType, row);
            if (bestValidity == null || rank(v) < rank(bestValidity)) {
                bestValidity = v;
                best = row;
            }
        }

        return new GovVerifyResult(
                certType.value(),
                certNo,
                nowIso(),
                true,
                bestValidity,
                certType == CertificationType.ORGANIC ? text(best, "Status") : null,
                normalizeDate(text(best, expiryFieldOf(certType))),
                text(best, holderNameFieldOf(certType)),
                text(best, holderAddressFieldOf(certType)),
                issuerNameFieldOf(certType) == null ? null : text(best, issuerNameFieldOf(certType)),
                buildProductSummary(certType, data),
                matchedCount,
                buildExcerpt(certType, data));
    }

    /**
     * 單列結論推導（§8.5）。
     *
     * <ul>
     *   <li>3701：讀權威 {@code Status} 欄 ＋ 效期</li>
     *   <li>3702：<b>無狀態欄</b> ⇒ 只能靠 {@code ValidDate}</li>
     *   <li>3703：<b>無狀態亦無效期</b> ⇒ 查得到即 VALID</li>
     * </ul>
     *
     * <p>判讀不出來（狀態值不在已知值域、或效期無法解析）→ {@code UNKNOWN}：
     * 那不是「無效」也不是「有效」，不寫快取、也不觸發自動核准。
     */
    private Validity deriveRowValidity(CertificationType certType, JsonNode row) {
        switch (certType) {
            case ORGANIC: {
                String status = text(row, "Status");
                if (status == null) {
                    return Validity.UNKNOWN;
                }
                switch (status) {
                    case "通過":
                        return notExpired(text(row, "EffectiveDate"));
                    case "結束":
                    case "終止":
                        return Validity.TERMINATED;
                    case "暫終":
                        return Validity.SUSPENDED;
                    default:
                        return Validity.UNKNOWN;
                }
            }
            case TAP:
                return notExpired(text(row, "ValidDate"));
            case CAS:
                // 無狀態亦無效期 ⇒ found 即 VALID。
                // ⚠️ 這只證明「該工廠具 CAS 資格」，不證明「這個商品是 CAS 產品」
                // ⇒ 3703 一律人工審核（§8.5），不得自動核准。
                return Validity.VALID;
            default:
                return Validity.UNKNOWN;
        }
    }

    /**
     * 效期未過 → VALID；已過 → EXPIRED；無法解析 → UNKNOWN。效期當日仍算有效。
     *
     * <p>「今天」以 {@link #GOV_DATA_ZONE}（台北）換算——政府資料給的是台灣在地日期，
     * 用 UTC 問今天會在台北的每天 00:00–08:00 拿到前一天，見 {@link #GOV_DATA_ZONE}。
     */
    private Validity notExpired(String raw) {
        LocalDate d = parseGovDate(raw);
        if (d == null) {
            return Validity.UNKNOWN;
        }
        LocalDate today = LocalDate.ofInstant(clock.instant(), GOV_DATA_ZONE);
        return d.isBefore(today) ? Validity.EXPIRED : Validity.VALID;
    }

    /**
     * 多列時取「最有利」的一列作為整體結論。
     *
     * <p>3701 一個編號對到 1–2 列（同一張證書的不同品項／行為別），只要有一列仍在有效狀態，
     * 該證書即為有效；3703 的 N 列全部同一張工廠標誌。
     */
    private static int rank(Validity v) {
        switch (v) {
            case VALID: return 0;
            case SUSPENDED: return 1;
            case EXPIRED: return 2;
            case TERMINATED: return 3;
            default: return 4; // UNKNOWN / NOT_FOUND
        }
    }

    /**
     * 防線 3：我方截斷 payload。
     *
     * <p>{@code $top} 實測被伺服器忽略（帶 {@code $top=1} 仍回 2 筆），因此大小只能由我方保證。
     */
    private ArrayNode buildExcerpt(CertificationType certType, JsonNode data) {
        ArrayNode out = objectMapper.createArrayNode();
        int rows = Math.min(data.size(), MAX_EXCERPT_ROWS);
        for (int i = 0; i < rows; i++) {
            JsonNode row = data.get(i);
            if (row == null || !row.isObject()) {
                continue;
            }
            ObjectNode copy = row.deepCopy();
            if (certType == CertificationType.TAP) {
                for (String drop : TAP_DROP_FIELDS) {
                    copy.remove(drop);
                }
                for (String arrayField : TAP_NESTED_ARRAYS) {
                    JsonNode nested = copy.get(arrayField);
                    if (nested != null && nested.isArray() && nested.size() > MAX_NESTED_ITEMS) {
                        ArrayNode trimmed = objectMapper.createArrayNode();
                        for (int j = 0; j < MAX_NESTED_ITEMS; j++) {
                            trimmed.add(nested.get(j));
                        }
                        copy.set(arrayField, trimmed);
                    }
                }
            }
            out.add(copy);
        }
        return out;
    }

    /** 跨列去重聚合品項摘要，並限制長度（3703 一個編號可對到 173 個產品）。 */
    private String buildProductSummary(CertificationType certType, JsonNode data) {
        Set<String> parts = new LinkedHashSet<>();
        for (JsonNode row : data) {
            for (String field : productFieldsOf(certType)) {
                String v = text(row, field);
                if (v != null) {
                    parts.add(v);
                }
            }
            if (parts.size() >= MAX_PRODUCT_SUMMARY_ITEMS) {
                break;
            }
        }
        if (parts.isEmpty()) {
            return null;
        }
        List<String> capped = new ArrayList<>(parts).subList(0, Math.min(parts.size(), MAX_PRODUCT_SUMMARY_ITEMS));
        String joined = String.join("、", capped);
        return joined.length() > MAX_PRODUCT_SUMMARY_LENGTH
                ? joined.substring(0, MAX_PRODUCT_SUMMARY_LENGTH)
                : joined;
    }

    // ==================== 欄位映射（§8.1）====================

    /** 防線 2 的編號回聲欄位。 */
    private static String echoFieldOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return "CertOrganicSn";
            case TAP: return "Tracecode";
            case CAS: return "Emblem_ID";
            default: throw new IllegalArgumentException("不支援的認證類型: " + certType.value());
        }
    }

    private static String expiryFieldOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return "EffectiveDate";
            case TAP: return "ValidDate";
            default: return "__none__"; // 3703 無效期欄位
        }
    }

    private static String holderNameFieldOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return "Name";
            case TAP: return "Producer";
            default: return "Factory_CName";
        }
    }

    private static String holderAddressFieldOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return "Address";
            case TAP: return "Place";
            default: return "Factory_Address";
        }
    }

    /** 3703 沒有驗證機構欄位。 */
    private static String issuerNameFieldOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return "CompanyName";
            case TAP: return "CertificationName";
            default: return null;
        }
    }

    private static List<String> productFieldsOf(CertificationType certType) {
        switch (certType) {
            case ORGANIC: return List.of("Products", "ContainCrops");
            case TAP: return List.of("ProductName");
            default: return List.of("Product_Name", "PType_Name");
        }
    }

    // ==================== 快取序列化 ====================

    private void writeCache(String cacheKey, GovVerifyResult result) {
        long ttl = ttlSecondsFor(result.validity());
        if (ttl <= 0) {
            // UNKNOWN：我方或上游故障，不是查驗結論。快取起來會把一次故障放大成一段時間的錯誤答案。
            log.warn("cacheSkipUnknown", "查驗結論為 UNKNOWN，不寫快取: certType=" + result.certType());
            return;
        }
        try {
            redisTemplate.opsForValue().set(cacheKey, toCacheJson(result), ttl, TimeUnit.SECONDS);
        } catch (DataAccessException e) {
            log.warn("cacheWriteUnavailable", "Redis 失聯，認證查驗結果快取寫回略過: " + e.getMessage());
        } catch (Exception e) {
            log.warn("cacheWriteFail", "認證查驗結果快取寫回失敗: " + e.getClass().getSimpleName());
        }
    }

    /** TTL 分級（§8.3.1）：VALID 7 天／NOT_FOUND 1 小時／已失效類 1 天／UNKNOWN 不寫。 */
    long ttlSecondsFor(Validity validity) {
        switch (validity) {
            case VALID:
                return govApiProperties.getCacheTtlExists();
            case NOT_FOUND:
                // 1 小時：避免賣家打錯字之後被自己的錯誤快取擋住一整天。
                return govApiProperties.getCacheTtlNotExists();
            case EXPIRED:
            case TERMINATED:
            case SUSPENDED:
                return govApiProperties.getCacheTtlInvalid();
            default:
                return -1;
        }
    }

    String toCacheJson(GovVerifyResult r) {
        ObjectNode n = objectMapper.createObjectNode();
        n.put("certType", r.certType());
        n.put("certNo", r.certNo());
        n.put("queriedAt", r.queriedAt());
        n.put("found", r.found());
        n.put("validity", r.validity() == null ? Validity.UNKNOWN.name() : r.validity().name());
        n.put("statusRaw", r.statusRaw());
        n.put("expiresOn", r.expiresOn());
        n.put("holderName", r.holderName());
        n.put("holderAddress", r.holderAddress());
        n.put("issuerName", r.issuerName());
        n.put("productSummary", r.productSummary());
        n.put("matchedCount", r.matchedCount());
        n.set("rawExcerpt", r.rawExcerpt() == null ? objectMapper.createArrayNode() : r.rawExcerpt());
        return n.toString();
    }

    /** 快取內容無法判讀時回 null（＝視為未命中），不得讓壞快取變成查驗結論。 */
    GovVerifyResult fromCacheJson(String json) {
        try {
            JsonNode n = objectMapper.readTree(json);
            if (n == null || !n.isObject()) {
                return null;
            }
            Validity validity = Validity.valueOf(n.path("validity").asText(Validity.UNKNOWN.name()));
            return new GovVerifyResult(
                    text(n, "certType"),
                    text(n, "certNo"),
                    text(n, "queriedAt"),
                    n.path("found").asBoolean(false),
                    validity,
                    text(n, "statusRaw"),
                    text(n, "expiresOn"),
                    text(n, "holderName"),
                    text(n, "holderAddress"),
                    text(n, "issuerName"),
                    text(n, "productSummary"),
                    n.path("matchedCount").asInt(0),
                    n.has("rawExcerpt") ? n.get("rawExcerpt") : objectMapper.createArrayNode());
        } catch (Exception e) {
            log.warn("cacheDecodeFail", "認證查驗快取內容無法判讀，視為未命中: " + e.getClass().getSimpleName());
            return null;
        }
    }

    // ==================== 小工具 ====================

    private static String text(JsonNode node, String field) {
        if (node == null || field == null) {
            return null;
        }
        JsonNode v = node.get(field);
        if (v == null || v.isNull()) {
            return null;
        }
        String s = v.asText("").trim();
        return s.isEmpty() ? null : s;
    }

    private static LocalDate parseGovDate(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        String s = raw.trim();
        try {
            return LocalDate.parse(s, GOV_DATE);
        } catch (Exception ignored) {
            // fallthrough
        }
        try {
            return LocalDate.parse(s);
        } catch (Exception ignored) {
            return null;
        }
    }

    /** 可解析時正規化成 {@code yyyy-MM-dd}，否則原樣保留（不擅自丟棄上游資訊）。 */
    private static String normalizeDate(String raw) {
        LocalDate d = parseGovDate(raw);
        return d == null ? raw : d.toString();
    }

    private static String nowIso() {
        return Instant.now().truncatedTo(ChronoUnit.SECONDS).toString();
    }

    /**
     * 依認證類型建構政府 API URL
     */
    private String buildGovApiUrl(CertificationType certType, String certNo) {
        String baseUrl;

        switch (certType) {
            case ORGANIC:
                baseUrl = govApiProperties.getOrganicUrl();
                break;
            case TAP:
                baseUrl = govApiProperties.getTapUrl();
                break;
            case CAS:
                baseUrl = govApiProperties.getCasUrl();
                break;
            default:
                throw new IllegalArgumentException("不支援的認證類型: " + certType.value());
        }
        // 查詢參數名與防線 2 的回聲欄位<b>同一個常數</b>：兩者若各寫一份，參數名打錯時
        // 校驗欄位也會跟著錯，防線 2 就白做了。
        String paramName = echoFieldOf(certType);

        // certNo 已由 isValidCertNo 收斂到 RFC 3986 unreserved 字元，可直接串接；
        // 若日後放寬白名單，這裡必須改成 percent-encoding。
        StringBuilder sb = new StringBuilder(baseUrl);
        sb.append("?").append(paramName).append("=").append(certNo);
        // ⚠️ $top 實測被伺服器忽略（帶 $top=1 仍回 2 筆），保留只為省流量，
        //    payload 大小的保證在防線 3，不在這裡。
        sb.append("&$top=1");

        // 加入 API Key
        String apiKey = govApiProperties.getApiKey1();
        if (apiKey != null && !apiKey.isEmpty()) {
            sb.append("&api_key=").append(apiKey);
        }

        return sb.toString();
    }

    /**
     * 建構快取 Key: gov:certv2:{type}:{hash}
     */
    private String buildCacheKey(CertificationType certType, String certNo) {
        return CACHE_PREFIX + certType.value() + ":" + sha256Short(certNo);
    }

    /**
     * SHA-256 取前 16 碼
     */
    private String sha256Short(String input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(input.getBytes(StandardCharsets.UTF_8));
            StringBuilder hex = new StringBuilder();
            for (byte b : hash) {
                hex.append(String.format("%02x", b));
            }
            return hex.substring(0, 16);
        } catch (Exception e) {
            // SHA-256 是 JDK 必備演算法，理論上不會拋出 — 走到這裡代表 JVM 損毀
            throw new IllegalStateException("SHA-256 not supported by JVM", e);
        }
    }

    /**
     * 政府 API 不可用例外（timeout / 連線失敗 / 4xx / 5xx / {@code RS != "OK"} /
     * 編號自我校驗失敗 — 統一視為「不可用」，對應 {@code gov_verify_state = UNAVAILABLE}）
     */
    public static class GovApiUnavailableException extends RuntimeException {
        public GovApiUnavailableException(String message) {
            super(message);
        }
    }

    /**
     * 政府 API 回應格式異常
     */
    public static class GovApiResponseException extends RuntimeException {
        public GovApiResponseException(String message) {
            super(message);
        }
    }
}
