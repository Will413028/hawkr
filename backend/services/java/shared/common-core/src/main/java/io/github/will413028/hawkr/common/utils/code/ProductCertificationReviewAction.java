package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 認證申請稽核事件（append-only）記錄的動作。⚠️ 本值域是**動作（動詞）**，狀態（名詞）是 `ProductCertificationStatus`，兩者刻意不收斂成同一個字——`APPROVE` 與 `AUTO_APPROVE` 都落到 `APPROVED` 狀態，但「誰放行的」正是自動核准路徑最需要事後可稽核的資訊，合併就查不出來了。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationReviewAction implements CodeMeta {

    SUBMIT("SUBMIT", "SUBMIT", "送出申請", "建立事件，`from_status` 為 null、`to_status` 為 `SUBMITTED`"),
    AUTO_APPROVE("AUTO_APPROVE", "AUTO_APPROVE", "系統自動核准", "`actor_role='SYSTEM'`、`actor_account_id` 為 null。**必須寫事件**：缺了它事後查不出「這張標章是誰放行的」"),
    APPROVE("APPROVE", "APPROVE", "管理員核准", "`actor_role='ADMIN'`，帶 `expectedVersion`"),
    REJECT("REJECT", "REJECT", "管理員駁回", "`reason_code` 必填"),
    WITHDRAW("WITHDRAW", "WITHDRAW", "撤回宣稱", "兩種來源：① 賣家主動撤回（`actor_role='SELLER'`，`actor_account_id` 是登入 principal）② **商品被軟刪的連帶撤回**（`actor_role='SYSTEM'`、`actor_account_id=null`，#2820）。②之所以不是 SELLER：賣家刪的是商品、不是撤回申請，撤回是平台施加的連帶結果，與 `SUPERSEDE` 同構；且 `DeleteProductRequest` 只帶 `store_id`，而 `SYSTEM` 明文「不得拿 store_id 頂替」"),
    SUPERSEDE("SUPERSEDE", "SUPERSEDE", "被新號碼取代", "`actor_role='SYSTEM'`，由核准新號碼的第一步連帶寫入"),
    REVOKE("REVOKE", "REVOKE", "管理員撤銷", "`reason_code` 必填；標章同時從公開投影移除");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationReviewAction(String code, String value, String label, String description) {
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

    private static final Map<String, ProductCertificationReviewAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationReviewAction::code, e -> e));

    private static final Map<String, ProductCertificationReviewAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationReviewAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationReviewAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationReviewAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
