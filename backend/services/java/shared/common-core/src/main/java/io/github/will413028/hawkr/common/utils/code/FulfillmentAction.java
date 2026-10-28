package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * **配送／履約軸**的領域動作值域，27 值。🔴 **fail-closed**：表上沒列的 action 一律不可用，前端只准 fallback 到 `view_only`，**不得自行猜測或退化成其他動作**。🔴 **與 provider capability 是兩層**：capability 回答「這個 provider 的 API 有沒有這個能力」，本 enum 的動作回答「這顆按鈕現在能不能按」；銜接兩層的是每個 action 自己宣告的 `requiresProviderCall`（且逐 provider 可覆寫）——`requiresProviderCall=false` 的動作（如 3004 的 handover／cancel、3005 L0 的取消）**不因 capability 為 empty 而消失**。URL 一律走 `allowedActions[].href`，前端不得自行組路徑。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FulfillmentAction implements CodeMeta {

    DISPATCH("DISPATCH", "dispatch", "派車／叫件", "配送推進群。3001 黑貓叫件、3003 Uber 建單即派車"),
    HANDOVER("HANDOVER", "handover", "回報交運", "配送推進群。B2C 交倉／自送出發，`requiresProviderCall=false`"),
    CANCEL("CANCEL", "cancel", "取消配送單", "配送推進群。🔴 四個 provider 中只有 3003 的 `requiresProviderCall=true`；其餘寫 992 或本地終態。"),
    RESUBMIT("RESUBMIT", "resubmit", "重新建單", "配送推進群。只有 990 建單失敗的配送單可重建"),
    UPDATE("UPDATE", "update", "修改配送資料", "配送推進群。⚠️ 在 provider 的 update 契約到位前**恆 disabled**"),
    PRINT_LABEL("PRINT_LABEL", "print_label", "列印標籤", "標籤群。3004 為批次型（單批上限與分組鍵由 capability 攜帶，呼叫端不得寫死數字）"),
    REPRINT_LABEL("REPRINT_LABEL", "reprint_label", "重印標籤", "標籤群。🔴 不得整批重印"),
    ASSIGN_BATCH("ASSIGN_BATCH", "assign_batch", "加入批次", "批次群（3004）。provider 端無對應 API，`requiresProviderCall` 恆 false"),
    REMOVE_FROM_BATCH("REMOVE_FROM_BATCH", "remove_from_batch", "移出批次", "批次群。同上"),
    SCAN_BIND("SCAN_BIND", "scan_bind", "掃描綁定", "批次群。同上"),
    CLOSE_BATCH("CLOSE_BATCH", "close_batch", "封箱關批", "批次群。同上"),
    TRACK("TRACK", "track", "查詢物流狀態", "追蹤與證明群"),
    VIEW_POD("VIEW_POD", "view_pod", "檢視送達證明", "追蹤與證明群。⚠️ 全值域統一 snake_case，不使用 camelCase"),
    UPLOAD_POD("UPLOAD_POD", "upload_pod", "上傳送達證明", "追蹤與證明群。3005 L0 由賣家上傳"),
    REQUERY("REQUERY", "requery", "重新查詢", "追蹤與證明群。主動向 provider 回查"),
    REPORT_PROGRESS("REPORT_PROGRESS", "report_progress", "回報配送進度", "賣家回報群（3005 三級共用），`requiresProviderCall` 恆 false"),
    REPORT_DELIVERED("REPORT_DELIVERED", "report_delivered", "回報已送達", "賣家回報群。同上"),
    REPORT_EXCEPTION("REPORT_EXCEPTION", "report_exception", "回報配送異常", "賣家回報群。同上"),
    ARRANGE_RETURN("ARRANGE_RETURN", "arrange_return", "安排退貨物流", "逆向與門市群"),
    ACCEPT_RETURN("ACCEPT_RETURN", "accept_return", "接受退貨", "逆向與門市群。🔴 冷凍逆向（3203）一律移除本動作"),
    SELLER_PICKUP("SELLER_PICKUP", "seller_pickup", "賣家領回", "逆向與門市群"),
    RESELECT_STORE("RESELECT_STORE", "reselect_store", "重選門市", "逆向與門市群。門市關轉（81）時使用"),
    OPEN_CASE("OPEN_CASE", "open_case", "建立異常案件", "ADM 群"),
    CLAIM("CLAIM", "claim", "受理理賠", "ADM 群。🔴 是**理賠**（993 CLAIM_PENDING），**不是案件認領**——ADM 文案必須寫「受理理賠」，寫「認領」會與不存在的認領動作混淆"),
    ADMIN_RESOLVE("ADMIN_RESOLVE", "admin_resolve", "處置提案", "ADM 群"),
    ADMIN_CLOSE_CASE("ADMIN_CLOSE_CASE", "admin_close_case", "關閉案件", "ADM 群"),
    VIEW_ONLY("VIEW_ONLY", "view_only", "僅檢視", "兜底群。🔴 **唯一允許的 fallback 值**");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FulfillmentAction(String code, String value, String label, String description) {
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

    private static final Map<String, FulfillmentAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentAction::code, e -> e));

    private static final Map<String, FulfillmentAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FulfillmentAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FulfillmentAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
