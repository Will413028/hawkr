package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 超商出貨單列印 / 宅配託運單取號的外部呼叫四態狀態機。賣家觸發後寫入 `REQUESTED`，PayUni 同步 API 失敗轉 `FAILED`，PayUni Print Webhook（`ApiType=Print`，超商）或同步 PDF 下載成功（宅配）轉 `SUCCEEDED`。共用同一狀態機處理超商與宅配兩條列印路徑。前端依此判斷「重列印」「重試」按鈕可用性，後端排程依 `REQUESTED 過久 / FAILED` 觸發補拉與重試。詳見 06-Shipping 設計書 §2.4。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LabelPrintStatus implements CodeMeta {

    NONE("NONE", "4101", "未列印", "物流單建立後預設值；尚未觸發任何列印呼叫"),
    REQUESTED("REQUESTED", "4102", "列印中", "已呼叫 PayUni 列印 API，等待 Print Webhook 回填代碼（超商）或 PDF 下載（宅配）"),
    SUCCEEDED("SUCCEEDED", "4103", "已列印", "PayUni Print Webhook 回填三段碼（超商）或 PDF 下載成功並上傳 OSS（宅配）"),
    FAILED("FAILED", "4104", "列印失敗", "PayUni 同步 API 拋例外，或 reconciliation 確認 PayUni 端無此 ShipTradeNo");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LabelPrintStatus(String code, String value, String label, String description) {
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

    private static final Map<String, LabelPrintStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LabelPrintStatus::code, e -> e));

    private static final Map<String, LabelPrintStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LabelPrintStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LabelPrintStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LabelPrintStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
