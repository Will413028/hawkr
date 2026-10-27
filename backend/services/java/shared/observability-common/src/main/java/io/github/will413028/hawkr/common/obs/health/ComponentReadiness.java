package io.github.will413028.hawkr.common.obs.health;

/**
 * 元件就緒契約。
 *
 * 任何「啟動時載入快取 / 啟動後初始化才能正確提供服務」的元件，必須實作此介面，
 * 否則 K8s readiness probe 會在 cache 尚未 warm 時就將流量導入，產生誤導性錯誤
 * （例如冷啟動視窗的結帳 ALL_SHIPPING_EXCEEDED）。
 *
 * 自動掃描：{@link ComponentReadinessHealthIndicator} 會收集所有實作此介面的
 * Spring bean，任一回傳 false 則 health endpoint 報 DOWN。
 *
 * 詳細規範見 docs/開發規範/Java 微服務開發規範.md §8.7。
 */
public interface ComponentReadiness {

    /** 元件是否已就緒接受流量。 */
    boolean isReady();

    /** 元件名稱（用於 health endpoint details 顯示）。 */
    String name();
}
