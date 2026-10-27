package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Media 端 import_asset 表的 5 態狀態機。終態為 FINALIZED / FAILED / EXPIRED。詳見 [40 號設計書 §2.4](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#24-importassetmedia-狀態機media-端-import_assetstatus)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportAssetMediaStatus implements CodeMeta {

    DOWNLOADING("DOWNLOADING", "9401", "下載中", "Media 正在 fetch 外部 URL"),
    UPLOADED("UPLOADED", "9402", "已上傳", "已落 OSS tmp 路徑"),
    FINALIZED("FINALIZED", "9403", "已轉正", "FinalizeImportAsset 完成，OSS 已搬到正式 refType 路徑（終態）"),
    FAILED("FAILED", "9404", "下載失敗", "SSRF 拒絕 / size 超限 / timeout / 404 等（終態）"),
    EXPIRED("EXPIRED", "9405", "已過期", "ImportJob 過期或 CANCELLED 觸發清理；OSS object 已刪（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportAssetMediaStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ImportAssetMediaStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetMediaStatus::code, e -> e));

    private static final Map<String, ImportAssetMediaStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetMediaStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportAssetMediaStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportAssetMediaStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
