package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 圖片下載失敗的細分原因，由 Media `ImportDownloadAsset` 從 SafeFetchException reason 對應，用於前端統一翻譯。詳見 [40 號設計書 §10.1](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#101-守則) 與 [08 號設計書 §7.7](../詳細設計書/08-多媒體微服務（Media%20Service）詳細設計書.md#77-safefetch-模組整合)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportAssetFailureReason implements CodeMeta {

    SSRF_BLOCKED("SSRF_BLOCKED", "SSRF_BLOCKED", "違反安全規範", "命中 IP 黑名單 / 內部網域 / 跨域 redirect 等"),
    PROTOCOL_NOT_ALLOWED("PROTOCOL_NOT_ALLOWED", "PROTOCOL_NOT_ALLOWED", "協定不允許", "非 https"),
    SIZE_EXCEEDED("SIZE_EXCEEDED", "SIZE_EXCEEDED", "檔案過大", "超過 10 MB"),
    TIMEOUT("TIMEOUT", "TIMEOUT", "連線逾時", "connect 5s 或 read 30s 超時"),
    NOT_FOUND("NOT_FOUND", "NOT_FOUND", "找不到資源", "HTTP 404"),
    UNSUPPORTED_TYPE("UNSUPPORTED_TYPE", "UNSUPPORTED_TYPE", "不支援的檔案類型", "content-type 不在白名單"),
    FETCH_ERROR("FETCH_ERROR", "FETCH_ERROR", "其他下載錯誤", "TLS 驗證失敗等"),
    MEDIA_DRIFT("MEDIA_DRIFT", "MEDIA_DRIFT", "Media 端不一致", "人工修復用（Import 端 UPLOADED 但 Media 找不到）"),
    CALLBACK_TIMEOUT("CALLBACK_TIMEOUT", "CALLBACK_TIMEOUT", "Callback 逾時", "async 下載 callback 30 分鐘未回，由 ProductImportGcJob 短週期掃描強制標 FAILED；詳見 40 號 §5.9");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportAssetFailureReason(String code, String value, String label, String description) {
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

    private static final Map<String, ImportAssetFailureReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetFailureReason::code, e -> e));

    private static final Map<String, ImportAssetFailureReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetFailureReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportAssetFailureReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportAssetFailureReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
