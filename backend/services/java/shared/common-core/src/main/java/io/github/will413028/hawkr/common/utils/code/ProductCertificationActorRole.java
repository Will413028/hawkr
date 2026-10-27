package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 認證申請稽核事件（append-only）的**動作發起者身分**。⚠️ 與 `ProductCertificationSource` 是兩件事、且**不是一對一**：source 記的是「這筆申請從哪條寫入路徑產生」，本值域記的是「這個事件是誰做的」。`actorRoleForSource` 把四種 source 收斂成兩種 role（`SELLER` → `SELLER`、`DELEGATED_ADMIN` → `ADMIN`、其餘一律 `SYSTEM`），而 `ADMIN` 還會由 approve／reject／revoke 這些**根本沒有 source 概念**的裁決動作產生 ⇒ 拿 source 反推 role 會漏掉全部人工裁決事件。與 `ProductCertificationReviewAction` 的關係是「誰（role）做了什麼（action）」，兩者必須並看：`AUTO_APPROVE` 恆為 `SYSTEM`、`APPROVE` 恆為 `ADMIN`，合併任一邊都會查不出「這張標章是誰放行的」。⚠️ **`WITHDRAW` 是唯一 role 不唯一的 action**：賣家主動撤回是 `SELLER`、商品軟刪的連帶撤回是 `SYSTEM`（#2820）⇒ 查「賣家撤回了哪些宣稱」必須同時篩 `action=WITHDRAW AND actor_role='SELLER'`，只篩 action 會把平台的連帶撤回也算進去。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationActorRole implements CodeMeta {

    SYSTEM("SYSTEM", "SYSTEM", "系統", "背景查驗自動核准（`AUTO_APPROVE`）、被新號碼取代（`SUPERSEDE`），以及匯入／seed／存量遷移路徑送出的申請。`actor_account_id` 必為 null——沒有具體的人在按按鈕，不得拿 `store_id` 頂替"),
    SELLER("SELLER", "SELLER", "賣家", "賣家本人送出（`SUBMIT`）或撤回（`WITHDRAW`）；`actor_account_id` 是賣家的登入 principal"),
    ADMIN("ADMIN", "ADMIN", "管理員", "管理員核准／駁回／撤銷，以及代行工作階段中代替賣家送出（`source = DELEGATED_ADMIN`）。`actor_account_id` 是**管理員本人**，不是被代行的賣家——這正是代行路徑事後追責的唯一依據");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationActorRole(String code, String value, String label, String description) {
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

    private static final Map<String, ProductCertificationActorRole> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationActorRole::code, e -> e));

    private static final Map<String, ProductCertificationActorRole> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationActorRole::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationActorRole fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationActorRole fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
