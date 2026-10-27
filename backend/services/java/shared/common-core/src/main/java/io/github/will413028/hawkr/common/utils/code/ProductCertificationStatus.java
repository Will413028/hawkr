package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品農產品認證申請的審核狀態。核心不變式是「**只有 `APPROVED` 會進入 `product.certifications` 公開投影**」——賣家填了什麼不等於平台認可什麼。四個終態（`REJECTED`／`WITHDRAWN`／`SUPERSEDED`／`REVOKED`）不再轉出。`WITHDRAWN` 與 `REJECTED` 刻意分開：前者是賣家自己停止宣稱、後者是平台判定不成立，合併會讓稽核誣指平台駁回。`SUPERSEDED` 與 `WITHDRAWN` 也刻意分開：換號碼時舊項要退場但保留歷程，歸到 `WITHDRAWN` 會把「換號」記成「停止宣稱」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationStatus implements CodeMeta {

    SUBMITTED("SUBMITTED", "SUBMITTED", "審核中", "賣家已宣稱、平台尚未認可；**不出現在公開投影**。同一 `(product, type)` 最多一筆（partial unique `uk_pca_pending`）"),
    APPROVED("APPROVED", "APPROVED", "已通過", "平台已認可，**出現在公開投影**；投影重算的唯一輸入集合。同一 `(product, type)` 最多一筆（partial unique `uk_pca_active`）"),
    REJECTED("REJECTED", "REJECTED", "未通過", "ADM 判定不成立（終態）。`reason_code` 必填；重送同一組號碼會帶 `requires_manual_review=true`，不得因政府查驗剛好通過就繞過裁決"),
    WITHDRAWN("WITHDRAWN", "WITHDRAWN", "已撤回", "賣家停止宣稱（終態）：專用 withdraw 端點、該 type 在全量取代中完全沒送、或**商品被軟刪**（#2820，連帶撤回該商品全部 `SUBMITTED`；此路徑的事件 `actor_role=SYSTEM`，見 `業務類型碼.md` 的 `ProductCertificationReviewAction.WITHDRAW`）"),
    SUPERSEDED("SUPERSEDED", "SUPERSEDED", "已被新號碼取代", "被同 type 的新號碼取代（終態）。由系統在核准新號碼的第一步寫入，保留換號歷程"),
    REVOKED("REVOKED", "REVOKED", "已撤銷", "ADM 事後撤銷已公開的標章（終態），`reason_code` 必填。**唯一的治理手段**——政府 API 只驗「號碼存在」不驗「歸屬」，盜用他人證號會自動通過查驗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProductCertificationStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationStatus::code, e -> e));

    private static final Map<String, ProductCertificationStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
