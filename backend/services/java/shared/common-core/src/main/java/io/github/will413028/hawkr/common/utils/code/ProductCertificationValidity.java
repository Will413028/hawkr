package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 從政府開放資料**判讀出來的證書效力**——這張證書現在還算不算數。⚠️ 與 `ProductCertificationGovVerifyState` 是**兩個不同的值域，不得合併**：本值域講的是**證書本身**（政府資料的判讀結論），`ProductCertificationGovVerifyState` 講的是**我方的查驗流程狀態**（`PENDING` 還沒查、`UNSUPPORTED` 這型根本不查、`UNAVAILABLE` 上游故障——這三個沒有任何一個在描述證書效力）。兩者是**多對一**且**有損**的：`toSnapshot` 把 `EXPIRED`／`TERMINATED`／`SUSPENDED` 三值一律塌成 `gov_verify_state = EXPIRED` ⇒ 只看 `govVerifyState` 的人分不出「已終止」（不可逆）與「暫時終止」（可能恢復），而這兩者的處置完全不同。ADM 判斷「這張證書到底能不能核准」的主要依據是本值域，不是塌縮後的那一個。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationValidity implements CodeMeta {

    VALID("VALID", "VALID", "有效", "政府資料查得到，且狀態與效期都通過。⚠️ 只有 3701 有機會據此自動核准（3703 查得到只證明該工廠有 CAS 資格）"),
    EXPIRED("EXPIRED", "EXPIRED", "已過期", "政府資料有這張證書，但效期已過。效期當日仍算有效；「今天」以台北時區換算，不用 UTC"),
    TERMINATED("TERMINATED", "TERMINATED", "已終止", "3701 的 `Status` 為「結束」或「終止」。⚠️ 與 `SUSPENDED` 的差別是**不可逆**——不會自己恢復，賣家要重新取得新證書"),
    SUSPENDED("SUSPENDED", "SUSPENDED", "暫時終止", "3701 的 `Status` 為「暫終」。證書仍在、暫時停用，日後可能恢復 ⇒ 處置與 `TERMINATED` 不同（可請賣家補件或稍後重驗，不必要求換號）"),
    NOT_FOUND("NOT_FOUND", "NOT_FOUND", "查無此編號", "政府資料回應正常但查無此編號。**不是**上游故障——故障走例外路徑，落 `gov_verify_state = UNAVAILABLE`、不產生本值域的任何值"),
    UNKNOWN("UNKNOWN", "UNKNOWN", "無法判讀", "查得到列但判讀不出結論：`Status` 欄缺漏或不在已知值域、效期字串無法解析。🔴 **這是故障不是結論**——一律不寫快取（把一次故障快取起來等於把它放大成一段時間的錯誤答案），ADM 應按「重新查驗」重試，不得據此駁回");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationValidity(String code, String value, String label, String description) {
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

    private static final Map<String, ProductCertificationValidity> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationValidity::code, e -> e));

    private static final Map<String, ProductCertificationValidity> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationValidity::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationValidity fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationValidity fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
