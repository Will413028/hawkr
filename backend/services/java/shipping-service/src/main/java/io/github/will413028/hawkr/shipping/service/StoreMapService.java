package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.shipping.GoodsType;
import io.github.will413028.hawkr.payuni.shipping.LgsType;
import io.github.will413028.hawkr.payuni.shipping.PayUniShippingEndpoint;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.shipping.config.PayUniConfig;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;

/**
 * 超商門市地圖服務
 * <p>
 * 負責組裝 PayUni 門市地圖前景 API 所需的加密參數，以及解密門市選擇結果。
 * PayUni 門市地圖為前景 API（Form POST），瀏覽器直接提交加密表單到 PayUni，
 * 使用者選店後 PayUni 將結果導回 MapReturnURL。
 * <p>
 * 同時負責組裝 redirectUrl（WebView 開啟 URL）與 Deep Link URL，
 * Gateway 僅透傳 redirect URL 做 302 導向，不做任何業務邏輯。
 */
@Service
public class StoreMapService {

    private static final ObsLogger log = ObsLogger.forClass(StoreMapService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    /** 固定參數：冷凍（冷凍門市可處理所有溫層） */
    private static final String FIXED_GOODS_TYPE = GoodsType.FROZEN.code();
    /** 固定參數：C2C 物流 */
    private static final String FIXED_LGS_TYPE = LgsType.C2C.code();
    /** 固定參數：7-ELEVEN */
    private static final String FIXED_SHIP_TYPE = ShipType.SEVEN.code();
    /** 固定參數：本島含離島 */
    private static final String FIXED_MAP_TYPE = "2";
    /** 固定參數：僅回傳門市資訊，不建立物流單 */
    private static final String FIXED_TAG = "2";

    private final PayUniCryptoService cryptoService;
    private final PayUniConfig payUniConfig;
    private final StoreMapRedirectFormStore redirectFormStore;

    /**
     * 買家網頁前端 base URL，當 appType="web" 時用於組裝最終 302 出口
     * （與 payment §5.9 / aftersales 共用同一環境變數值）。
     */
    @Value("${hawkr.web.frontend-url:}")
    private String webFrontendUrl;

    public StoreMapService(PayUniCryptoService cryptoService, PayUniConfig payUniConfig,
                           StoreMapRedirectFormStore redirectFormStore) {
        this.cryptoService = cryptoService;
        this.payUniConfig = payUniConfig;
        this.redirectFormStore = redirectFormStore;
    }

    /**
     * 組裝門市地圖表單參數（加密後的 EncryptInfo + HashInfo + redirectUrl）
     *
     * @param accountId 使用者帳號 ID（作為 MerKeyNo 唯一識別）
     * @param appType   "buyer" 或 "seller"，用於決定導回的 URL Scheme
     * @param mobileTag "Y"=手機版(預設), "N"=PC版
     * @return 門市地圖表單資料（含 redirectUrl）
     */
    public StoreMapFormResult buildStoreMapParams(long accountId, String appType, String mobileTag) {
        String merId = payUniConfig.getStoreMapMerId();
        String returnUrl = buildMapReturnUrl(appType);

        // 組裝 PayUni 門市地圖內層參數
        Map<String, String> params = new LinkedHashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("MerKeyNo", String.valueOf(accountId));
        params.put("GoodsType", FIXED_GOODS_TYPE);
        params.put("LgsType", FIXED_LGS_TYPE);
        params.put("ShipType", FIXED_SHIP_TYPE);
        params.put("MapType", FIXED_MAP_TYPE);
        params.put("Tag", FIXED_TAG);
        params.put("MobileTag", mobileTag != null && !mobileTag.isEmpty() ? mobileTag : "Y");
        if (returnUrl != null && !returnUrl.isEmpty()) {
            params.put("MapReturnURL", returnUrl);
        }

        // 加密 + 簽章
        String encryptInfo = cryptoService.encrypt(params);
        String hashInfo = cryptoService.calculateHash(encryptInfo);

        String formAction = payUniConfig.getBaseUrl() + PayUniShippingEndpoint.PATH_STORE_MAP;
        String version = PayUniShippingEndpoint.VERSION_STORE_MAP;

        // 組裝 redirectUrl（canonical 表單存 Redis 換 opaque id → /store_map/redirect?id=xxx）
        String redirectUrl = buildRedirectUrl(formAction, merId, version, encryptInfo, hashInfo);

        log.info("StoreMapService.buildStoreMapParams.success",
                "accountId=" + accountId + ", appType=" + appType + ", merId=" + merId);

        return new StoreMapFormResult(formAction, merId, version, encryptInfo, hashInfo, redirectUrl);
    }

    /**
     * 解密門市選擇結果並組裝 Deep Link URL
     *
     * @param encryptInfo PayUni 回傳的 AES 加密字串
     * @param hashInfo    PayUni 回傳的 SHA256 簽章
     * @param appType     "buyer" 或 "seller"，決定 Deep Link Scheme
     * @return 解密結果（含門市資訊 + Deep Link URL）
     */
    public StoreMapDecryptResult decryptStoreMapResult(String encryptInfo, String hashInfo, String appType) {
        // 驗簽
        if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
            throw new IllegalArgumentException("門市地圖簽章驗證失敗");
        }

        // 解密
        Map<String, String> data = cryptoService.decrypt(encryptInfo);
        log.info("StoreMapService.decryptStoreMapResult.success", "解密門市資料: " + data);

        // 門市資訊在 MapJson 欄位（JSON 字串），需二次解析
        String storeId = "";
        String storeName = "";
        String address = "";
        String insularArea = "";
        String storeType = "";

        String mapJson = getOrEmpty(data, "MapJson");
        if (!mapJson.isEmpty()) {
            try {
                @SuppressWarnings("unchecked")
                Map<String, String> mapData = MAPPER.readValue(mapJson, Map.class);
                storeId = getOrEmpty(mapData, "StoreID");
                storeName = getOrEmpty(mapData, "StoreName");
                address = getOrEmpty(mapData, "Address");
                insularArea = getOrEmpty(mapData, "InsularArea");
                storeType = getOrEmpty(mapData, "StoreType");
            } catch (JsonProcessingException e) {
                log.error("StoreMapService.decryptStoreMapResult.error", "MapJson 解析失敗: " + e.getMessage(), e);
            }
        }

        // 組裝 Deep Link URL（由微服務決定 URL Scheme）
        String redirectUrl = buildDeepLink(appType, storeId, storeName, address, insularArea, storeType);

        return new StoreMapDecryptResult(storeId, storeName, address, insularArea, storeType, redirectUrl);
    }

    /**
     * 組裝錯誤導回 URL：
     * - appType="web" → {WEB_FRONTEND_URL}/store-map/return?error=...（瀏覽器同源頁面）
     * - appType="buyer"/"seller" → 對應 App URL Scheme deep link
     * 詳見設計書 06 §5.14.2「錯誤分支」。
     */
    public String buildErrorDeepLink(String appType, String errorMsg) {
        if ("web".equals(appType)) {
            return webBaseUrl() + "/store-map/return?error=" + encode(errorMsg);
        }
        String scheme = "seller".equals(appType) ? "hawkrseller" : "hawkrbuyer";
        return scheme + "://store_map/result?error=" + encode(errorMsg);
    }

    // ==================== 私有方法 ====================

    /**
     * 組裝 redirectUrl（canonical 表單存 Redis 換 opaque id → GATEWAY_URL/store_map/redirect?id=xxx）
     */
    private String buildRedirectUrl(String formAction, String merId, String version,
                                     String encryptInfo, String hashInfo) {
        String gatewayUrl = payUniConfig.getGatewayUrl();
        if (gatewayUrl == null || gatewayUrl.isEmpty()) {
            return "";
        }

        try {
            String id = redirectFormStore.store(new StoreMapRedirectFormStore.RedirectForm(
                    formAction, merId, version, encryptInfo, hashInfo));
            return gatewayUrl + "/store_map/redirect?id=" + id;
        } catch (JsonProcessingException e) {
            log.error("StoreMapService.buildRedirectUrl.error", "JSON 序列化失敗: " + e.getMessage(), e);
            return "";
        }
    }

    /**
     * 依 opaque id 一次性取回門市中轉頁 canonical 表單（供 Gateway redirect handler 使用）。
     *
     * @param redirectId opaque redirect id
     * @return canonical 表單，若查無 / 過期 / 已用則為 empty
     */
    public Optional<StoreMapRedirectFormStore.RedirectForm> consumeRedirectForm(String redirectId) {
        return redirectFormStore.consume(redirectId);
    }

    /**
     * 組裝門市選擇成功的導回 URL：
     * - appType="web" → {WEB_FRONTEND_URL}/store-map/return?storeId=...（瀏覽器同源頁面，由前端 postMessage 回 opener）
     * - appType="buyer"/"seller" → 對應 App URL Scheme deep link（WebView 攔截）
     * 詳見設計書 06 §5.14.2「最終出口分支矩陣」。
     */
    private String buildDeepLink(String appType, String storeId, String storeName,
                                   String address, String insularArea, String storeType) {
        String query = "?storeId=" + encode(storeId)
                + "&storeName=" + encode(storeName)
                + "&address=" + encode(address)
                + "&insularArea=" + encode(insularArea)
                + "&storeType=" + encode(storeType);
        if ("web".equals(appType)) {
            return webBaseUrl() + "/store-map/return" + query;
        }
        String scheme = "seller".equals(appType) ? "hawkrseller" : "hawkrbuyer";
        return scheme + "://store_map/result" + query;
    }

    /**
     * 取得買家網頁 base URL，去除尾端斜線，避免組出 `//store-map/return` 雙斜線。
     */
    private String webBaseUrl() {
        String base = webFrontendUrl != null ? webFrontendUrl : "";
        if (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        return base;
    }

    /**
     * 組裝 MapReturnURL（含 appType 參數，供 Gateway 解析並導向對應 APP）
     */
    private String buildMapReturnUrl(String appType) {
        String baseReturnUrl = payUniConfig.getStoreMapReturnUrl();
        if (baseReturnUrl == null || baseReturnUrl.isEmpty()) {
            return "";
        }
        return baseReturnUrl + "?app=" + (appType != null ? appType : "buyer");
    }

    private String getOrEmpty(Map<String, String> map, String key) {
        String value = map.get(key);
        return value != null ? value : "";
    }

    private String encode(String value) {
        if (value == null || value.isEmpty()) return "";
        return URLEncoder.encode(value, StandardCharsets.UTF_8);
    }

    /**
     * 門市地圖表單資料（加密後，含 redirectUrl）
     */
    public record StoreMapFormResult(
            String formAction,
            String merId,
            String version,
            String encryptInfo,
            String hashInfo,
            String redirectUrl
    ) {}

    /**
     * 門市選擇解密結果（含 Deep Link URL）
     */
    public record StoreMapDecryptResult(
            String storeId,
            String storeName,
            String address,
            String insularArea,
            String storeType,
            String redirectUrl
    ) {}
}
