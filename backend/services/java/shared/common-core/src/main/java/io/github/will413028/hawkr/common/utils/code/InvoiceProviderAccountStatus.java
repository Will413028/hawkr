package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家的 AMEGO 帳戶連線狀態（**11 個 DB 值 ＋ 2 個投影值 ＝ 13**）。🔴 **沒有 SUSPENDING**（2026-08-25 依 58 §7.3.4 刪除）：它是 DRAIN 停用（等既有 job 跑完再停用）的中間狀態，而 **DRAIN 整條分支已刪除**——端點與畫面兩側都沒有處置選項（端點 45 的 request 只有 `Reason`／`Confirmed`／`ExpectedVersion`）、它要解的問題已有解（在途 job 被打成 FAILED_PERMANENT 之後不是終態，恢復後由 ADM 逐筆安全重試），而它自帶一個反向風險（SUSPENDING 期間憑證仍可用，但按下停用最常見的理由正是「AppKey 外洩」）。⚠️ **不要因為「停用應該要溫和一點」就把它加回來**——那要同批動 58 §5 的兩個 drain 欄與兩條 CHECK、58 §7 的四列轉移、一支取消端點與一組 UI。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceProviderAccountStatus implements CodeMeta {

    NOT_CONFIGURED("NOT_CONFIGURED", "NOT_CONFIGURED", "未設定", "已建立列但未填憑證"),
    CREDENTIAL_SUBMITTED("CREDENTIAL_SUBMITTED", "CREDENTIAL_SUBMITTED", "憑證已送出", "統編與 AppKey 已寫入，尚未開始驗證"),
    CREDENTIAL_VERIFYING("CREDENTIAL_VERIFYING", "CREDENTIAL_VERIFYING", "驗證中", "正在打 AMEGO 驗簽章、統編與字軌"),
    ACTIVE("ACTIVE", "ACTIVE", "已啟用", "三項驗證全過且至少一組可用字軌；可設定自動開立切點"),
    INVALID_CREDENTIAL("INVALID_CREDENTIAL", "INVALID_CREDENTIAL", "憑證錯誤", "簽章或統編不符（AMEGO code 16／12）；賣家重新輸入即可修復"),
    PROVIDER_REJECTED("PROVIDER_REJECTED", "PROVIDER_REJECTED", "服務商拒絕", "帳號未啟用／停權／未申請 API（code 13／19／22）；需回光貿後台處理"),
    PROVIDER_UNAVAILABLE("PROVIDER_UNAVAILABLE", "PROVIDER_UNAVAILABLE", "服務商無法連線", "停機維護或逾時（code 10／18／21）；排入 15 分鐘一次自動重驗"),
    ROTATION_PENDING("ROTATION_PENDING", "ROTATION_PENDING", "憑證輪替中", "新舊 key 並存；驗證失敗一律保留原 key"),
    DEGRADED("DEGRADED", "DEGRADED", "降級中", "斷路器開路；operation 停在 RETRY_SCHEDULED 不進 SENDING"),
    SUSPENDED("SUSPENDED", "SUSPENDED", "已停用", "ADM 停用（P14，端點 45）寫入。**不是終態**：恢復走端點 66（P15，恢復前強制重驗、不得直接回 ACTIVE）、撤銷走端點 67（P16）"),
    REVOKED("REVOKED", "REVOKED", "已作廢", "金鑰永久作廢／賣家離場（**M4 唯一終態**）。唯一入口＝端點 67（P16）。⚠️ 它同時是 `uq_ipa_fingerprint` partial 述詞的排除值 ⇒ 賣家關店重開能否綁回同一組憑證，取決於本值真的到得了（58 §5 I-37 第二向）"),
    NOT_STARTED("NOT_STARTED", "NOT_STARTED", "尚未開始設定", "**投影值，不落 DB**：查無 provider account 列時對外回這個值。上線當天全體賣家都是它"),
    CONFIG_ERROR("CONFIG_ERROR", "CONFIG_ERROR", "配置錯誤", "**投影值，不落 DB**：有列但 status 為空字串或未知值，進 ADM A01 KPI");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceProviderAccountStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceProviderAccountStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceProviderAccountStatus::code, e -> e));

    private static final Map<String, InvoiceProviderAccountStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceProviderAccountStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceProviderAccountStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceProviderAccountStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
