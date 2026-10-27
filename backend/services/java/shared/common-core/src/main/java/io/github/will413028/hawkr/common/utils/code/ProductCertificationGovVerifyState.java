package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 對政府開放資料平台查驗該證書編號的結果狀態。⚠️ 本值域是**查驗結果**，與審核狀態 `ProductCertificationStatus` 是兩件事：`VERIFIED` 不等於 `APPROVED`（3703 CAS 查得到只證明該工廠有資格、不證明這個商品是 CAS 產品，一律人工審），`NOT_FOUND` 也不等於 `REJECTED`（證書可能在申請之後才登記上政府資料庫，ADM 可按「重新查驗」）。⚠️ 也**不要**與 `ProductCertificationValidity` 合併：那張表是「政府資料判讀出的證書效力」（含 `TERMINATED` 已終止／`SUSPENDED` 暫時終止），本值域是「我方查驗流程走到哪」（含 `PENDING`／`UNSUPPORTED`／`UNAVAILABLE` 這三個與證書效力無關的流程態）；本值域的 `EXPIRED` 是那三個失效值**塌縮後**的結果，有損，ADM 要判斷處置一律讀 `govVerifyPayload.validity`。🔴 **`UNAVAILABLE` 與 `NOT_FOUND` 絕不可合併**：政府 API 金鑰失效時回 `{"RS":"ERROR"}` 且 HTTP 200 無 `Data` 欄，舊實作把它判成「查無此證書」⇒ 金鑰一出事全站所有編號都被判為假的且無任何錯誤訊號（19 §8.4 防線 1 實測）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationGovVerifyState implements CodeMeta {

    PENDING("PENDING", "PENDING", "待查驗", "DB 預設值；背景查驗事件尚未處理完成"),
    VERIFIED("VERIFIED", "VERIFIED", "查驗通過", "政府資料查得到且推導出的 `validity = VALID`。⚠️ 只有 3701 有機（有權威 `Status` 欄＋效期＋編號一對一）會據此自動核准"),
    NOT_FOUND("NOT_FOUND", "NOT_FOUND", "查無此編號", "政府資料查無此編號。**不是**上游故障——上游故障一律落 `UNAVAILABLE`"),
    EXPIRED("EXPIRED", "EXPIRED", "證書已失效", "查得到但推導出 `EXPIRED`／`TERMINATED`／`SUSPENDED`；需人工審"),
    UNSUPPORTED("UNSUPPORTED", "UNSUPPORTED", "不支援線上查驗", "3704 HACCP／3705 ISO／3706 GMP 無政府開放資料介面，永遠人工審，不排背景查驗"),
    UNAVAILABLE("UNAVAILABLE", "UNAVAILABLE", "查驗服務不可用", "政府 API 逾時／連線失敗／回應的 `RS` 欄不是 OK／編號自我校驗失敗／斷路器命中。ADM 可稍後按「重新查驗」重試");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationGovVerifyState(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ProductCertificationGovVerifyState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationGovVerifyState::code, e -> e));

    private static final Map<String, ProductCertificationGovVerifyState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationGovVerifyState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationGovVerifyState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationGovVerifyState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
