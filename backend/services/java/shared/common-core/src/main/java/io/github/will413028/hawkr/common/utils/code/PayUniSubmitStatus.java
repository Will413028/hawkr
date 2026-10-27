package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家建店送 PayUni 設定 API（`/api/logistics/setting` / `/api/home_delivery/setting`）的外部呼叫四態狀態機。取代原 `payuni_submitted BOOLEAN`，並接管「宅配地址不可更新」「30 秒 cooldown 防 PayUni race」等業務規則的判定。前端依此判斷宅配地址欄位 readonly；後端建單時依此選用 DB 既有地址而非 request 地址。詳見 06-Shipping 設計書 §2.4 / §5.9 / §5.10。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayUniSubmitStatus implements CodeMeta {

    NONE("NONE", "4301", "未送 PayUni", "設定記錄初始值；尚未首次送 PayUni（無此記錄時等同此值）"),
    REQUESTED("REQUESTED", "4302", "送審中", "已呼叫 PayUni setting API，等待同步回應（短暫過渡狀態，因 PayUni setting 為純同步）"),
    SUCCEEDED("SUCCEEDED", "4303", "已送 PayUni", "首次或重新送 PayUni 成功；地址欄位後續不再可更新（宅配限制）"),
    FAILED("FAILED", "4304", "送審失敗", "PayUni 回傳失敗（如地址郵號不符）；不建立設定記錄（首次）或維持前次成功狀態（重送），由賣家修正後重試");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayUniSubmitStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PayUniSubmitStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniSubmitStatus::code, e -> e));

    private static final Map<String, PayUniSubmitStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniSubmitStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayUniSubmitStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayUniSubmitStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
