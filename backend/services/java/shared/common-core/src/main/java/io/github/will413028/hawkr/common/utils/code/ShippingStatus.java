package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 正規化配送域（normalized delivery）狀態碼，30 值。**號碼慣例：兩位數＝PayUni 原生值，三位數＝平台自訂值**——新增值一律取三位，佔用兩位號碼會在 PayUni 日後新增狀態碼時被 `fromValue()` 解析成語意完全不同但合法的值。每個 provider 都必須走顯式映射表（PayUni 那張是恆等映射但必須存在），provider 原生值另存 `shipments.provider_status`
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShippingStatus implements CodeMeta {

    UNPROCESSED("UNPROCESSED", "91", "未處理", "尚未取得出貨單編號"),
    PROCESSING("PROCESSING", "92", "處理中", "出貨單編號傳送至上游廠商確認中（僅限超商）"),
    PENDING_SHIPMENT("PENDING_SHIPMENT", "21", "待出貨", "等待商店出貨"),
    LOGISTICS_CHECK("LOGISTICS_CHECK", "22", "物流驗收", "物流中心驗收中（僅限超商）"),
    IN_TRANSIT("IN_TRANSIT", "31", "配送中", "安排出貨中"),
    WAITING_PICKUP("WAITING_PICKUP", "32", "待取貨", "包裹配達門市，今日 23:59 後將退回物流中心"),
    ABNORMAL("ABNORMAL", "33", "異常訂單", "配送過程異常"),
    PICKED_UP("PICKED_UP", "11", "已取貨", "消費者已取貨"),
    CANCELLED("CANCELLED", "41", "已取消", "訂單已被商店取消"),
    COMPENSATION("COMPENSATION", "43", "賠償訂單", "物流中心/黑貓確認賠償訂單產生"),
    PACKAGE_LOST("PACKAGE_LOST", "44", "包裹遺失", "門市/物流中心遺失包裹，協尋 18 天未果"),
    PACKAGE_ABANDONED("PACKAGE_ABANDONED", "46", "包裹拋棄", "賣家未取包裹且逾期未提供宅配資料（僅限超商 C2C）"),
    RETURN_GENERAL("RETURN_GENERAL", "51", "一般退貨", "退貨便/黑貓退貨"),
    BUYER_NOT_PICKED("BUYER_NOT_PICKED", "52", "買家未取", "買家超過取貨期限未取，產生退貨"),
    RETURN_TO_WAREHOUSE("RETURN_TO_WAREHOUSE", "53", "廠退", "包裹退回物流中心/黑貓集貨所"),
    SELLER_NOT_PICKED("SELLER_NOT_PICKED", "55", "賣家未取", "買家未取包裹配達退貨門市後，商店未於期限內領取（僅限超商 C2C）"),
    RETURN_BY_HOME_DELIVERY("RETURN_BY_HOME_DELIVERY", "56", "已轉宅配退回", "賣家未取包裹已交付黑貓宅配退回指定地址（僅限超商 C2C）"),
    STORE_TRANSFER("STORE_TRANSFER", "81", "門市關轉", "門市暫歇（裝修、改店號等），需在期限內重選門市"),
    PENDING_HOME_RETURN("PENDING_HOME_RETURN", "82", "待轉宅配退回", "包裹退回物流中心保管，將寄信提醒商店（僅限超商 C2C）"),
    TCAT_CALLED("TCAT_CALLED", "210", "已叫件", "callTcat 成功，等待黑貓司機取件"),
    CVS_CODE_RECEIVED("CVS_CODE_RECEIVED", "920", "已取得交貨便代碼", "超商 Print Webhook 已回填出貨便代碼"),
    SELLER_CONFIRMED("SELLER_CONFIRMED", "921", "賣家確認出貨", "超商賣家手動確認已出貨"),
    CREATE_FAILED("CREATE_FAILED", "990", "建單失敗", "建單重試後**確定**失敗（provider_status/message 落地供 Admin 閉環）。出口：Admin 重建→91／Admin 取消→41"),
    HANDED_OVER("HANDED_OVER", "930", "已交運", "已交運：Uber 取件完成。適用 3003"),
    COURIER_ASSIGNED("COURIER_ASSIGNED", "933", "已指派外送員", "已指派外送員、前往取件（對應 Uber 狀態值 pickup）。適用 3003"),
    CREATE_UNKNOWN("CREATE_UNKNOWN", "991", "建單結果未知", "建單呼叫逾時或非確定性錯誤——**不是失敗**。由回查排程收斂：查到成功→91／查到失敗→990／超出回查窗仍未知→41＋人工"),
    CANCEL_PENDING("CANCEL_PENDING", "992", "取消處理中", "取消申請已受理，待 provider 確認。適用 3002／3003／3005。**唯一允許回退到非終態的狀態**"),
    CLAIM_PENDING("CLAIM_PENDING", "993", "理賠受理中", "理賠受理中（遺失／毀損爭議）。適用 3001／3002。case 關閉：成立→43／不成立→33 或原異常值");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShippingStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ShippingStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingStatus::code, e -> e));

    private static final Map<String, ShippingStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShippingStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShippingStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
