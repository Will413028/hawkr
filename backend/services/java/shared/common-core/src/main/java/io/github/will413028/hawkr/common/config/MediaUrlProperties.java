package io.github.will413028.hawkr.common.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * 媒體 URL 組裝共用配置（49 號設計書 §3/§4.3）。
 * <p>
 * 統一管理所有微服務的媒體基礎 URL，域名唯一來源 {@code MEDIA_BASE_URL}，禁止各服務自行定義。
 * 由 MediaUrlAutoConfiguration 自動載入，微服務只需 @Autowired 即可使用。
 * DB／MQ／快取只存 objectKey，顯示時才由 {@link #buildUrl(String)} 組裝。
 */
@ConfigurationProperties(prefix = "hawkr.media")
public class MediaUrlProperties {

    /**
     * 媒體基礎 URL。
     * <p>
     * <b>沒有預設域名</b>（backend#2283）：媒體域名不做新舊相容——dev 一律
     * {@code https://test-media.example.com}、prod 一律 {@code https://media.example.com}，
     * 由 {@code MEDIA_BASE_URL} 注入。舊值 {@code https://cdn.example.com} 是退役中的舊 OSS
     * 網域，DNS 一撤就會讓漏設的服務靜默吐出死連結，已移除。
     * <p>
     * Go 側是缺值即 fail-fast（各 service 的 {@code Config.Validate()}），Java 側**刻意不是**：
     * 本 bean 由 {@code MediaUrlAutoConfiguration} 注入**全部 10 個 Java 服務**，其中只有 cart／
     * media／order／shipping 真的呼叫 {@link #buildUrl(String)}。把它變成啟動必要條件，等於讓
     * 六個從不讀這個值的服務因為一個與它們無關的環境變數而 CrashLoop。fail-fast 應該守在
     * 消費端，不是守在 autoconfiguration。空值時 {@link #buildUrl(String, String)} 回傳裸
     * objectKey（與 Go {@code pkg/mediaurl.BuildImageURL} 同語意），前端 buildImageUrl 會補上
     * 各自的 media base。
     */
    private String baseUrl = "";

    public String getBaseUrl() {
        return baseUrl;
    }

    public void setBaseUrl(String baseUrl) {
        this.baseUrl = baseUrl;
    }

    /**
     * 組裝原圖基底 URL（Image Gateway 契約 base + "/i/original/" + objectKey，49 號 §6.1/§6.5）。
     * imageUrl＝原圖基底 URL，走 Worker {@code /i/original/{objectKey}} 路徑。
     *
     * @param objectKey 儲存路徑（如 public/product/20260304/xxx.jpg）
     * @return 完整 URL（如 https://media.example.com/i/original/public/product/20260304/xxx.jpg），objectKey 為空時回傳空字串
     */
    public String buildUrl(String objectKey) {
        return buildUrl(objectKey, "original");
    }

    /**
     * 組裝 Image Gateway preset URL（base + "/i/" + preset + "/" + objectKey，對應 49 號 §4.4 preset 白名單）。
     * 切換至 Image Gateway（MEDIA_BASE_URL 指向 Worker 域名）後生效；空值與 http 前綴處理同 {@link #buildUrl(String)}。
     *
     * @param objectKey 儲存路徑
     * @param preset    preset 白名單名稱（avatar/thumb/card/detail/banner/chat/original）
     * @return preset URL，objectKey 為空時回傳空字串
     */
    public String buildUrl(String objectKey, String preset) {
        if (objectKey == null || objectKey.isEmpty()) {
            return "";
        }
        if (objectKey.startsWith("http://") || objectKey.startsWith("https://")) {
            return objectKey;
        }
        // baseUrl 未設 → 回傳裸 objectKey（對齊 Go pkg/mediaurl.BuildImageURL 的語意）。
        // 不可回傳 "/i/{preset}/{key}"：那是相對路徑，前端會再組一次而變成 /i/…/i/…。
        if (baseUrl == null || baseUrl.isEmpty()) {
            return objectKey.startsWith("/") ? objectKey.substring(1) : objectKey;
        }
        String base = baseUrl;
        if (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        String key = objectKey.startsWith("/") ? objectKey.substring(1) : objectKey;
        return base + "/i/" + preset + "/" + key;
    }
}
