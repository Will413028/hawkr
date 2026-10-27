package io.github.will413028.hawkr.product.config;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.context.annotation.Configuration;

/**
 * 農業部開放資料 API 配置
 */
@Configuration
@ConfigurationProperties(prefix = "hawkr.gov-api")
public class GovApiProperties {

    private String organicUrl = "https://data.moa.gov.tw/api/v1/TWOrganicAgricultureVerificationInformationType/";
    private String tapUrl = "https://data.moa.gov.tw/api/v1/TraceabilityType/";
    private String casUrl = "https://data.moa.gov.tw/api/v1/CASProductInquiryType/";
    private String apiKey1 = "";
    private int connectTimeout = 5000;
    private int readTimeout = 10000;

    /**
     * 背景查驗專用 read-timeout（毫秒，設計書 19 §8.7）。
     *
     * <p>3702 產銷履歷實測 11 次取樣落在 33–82 秒。設 70 秒會讓其中至少 4 筆逾時
     * ⇒ 背景查驗多半照樣拿不到結果、整套「背景補課」機制形同虛設。
     * 90 秒涵蓋實測最大值 81.9 秒 ＋ 約 10% 餘裕。
     *
     * <p>⚠️ 只給背景 consumer 用；同步 advisory 端點維持 {@link #readTimeout} 10 秒
     * （使用者在等畫面回應，不能讓他等 90 秒）。
     */
    private int backgroundReadTimeout = 90000;

    private long cacheTtlExists = 604800;      // VALID：7天（秒）
    private long cacheTtlNotExists = 3600;     // NOT_FOUND：1小時（秒），避免打錯字後被自己的錯誤快取擋住
    private long cacheTtlInvalid = 86400;      // EXPIRED / TERMINATED / SUSPENDED：1天（秒）

    public String getOrganicUrl() { return organicUrl; }
    public void setOrganicUrl(String organicUrl) { this.organicUrl = organicUrl; }

    public String getTapUrl() { return tapUrl; }
    public void setTapUrl(String tapUrl) { this.tapUrl = tapUrl; }

    public String getCasUrl() { return casUrl; }
    public void setCasUrl(String casUrl) { this.casUrl = casUrl; }

    public String getApiKey1() { return apiKey1; }
    public void setApiKey1(String apiKey1) { this.apiKey1 = apiKey1; }

    public int getConnectTimeout() { return connectTimeout; }
    public void setConnectTimeout(int connectTimeout) { this.connectTimeout = connectTimeout; }

    public int getReadTimeout() { return readTimeout; }
    public void setReadTimeout(int readTimeout) { this.readTimeout = readTimeout; }

    public int getBackgroundReadTimeout() { return backgroundReadTimeout; }
    public void setBackgroundReadTimeout(int backgroundReadTimeout) { this.backgroundReadTimeout = backgroundReadTimeout; }

    public long getCacheTtlExists() { return cacheTtlExists; }
    public void setCacheTtlExists(long cacheTtlExists) { this.cacheTtlExists = cacheTtlExists; }

    public long getCacheTtlNotExists() { return cacheTtlNotExists; }
    public void setCacheTtlNotExists(long cacheTtlNotExists) { this.cacheTtlNotExists = cacheTtlNotExists; }

    public long getCacheTtlInvalid() { return cacheTtlInvalid; }
    public void setCacheTtlInvalid(long cacheTtlInvalid) { this.cacheTtlInvalid = cacheTtlInvalid; }
}
