package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 物流貨態狀態碼
 * <p>
 * 嚴格依據 PayUni 物流 API 官方文件定義
 * 參考：詳細設計書/統一金流/統一物流官方相關API文檔.md
 */
public enum ShipStatusCode {

    /** 未處理 */
    UNPROCESSED("91", "未處理"),
    /** 處理中 */
    PROCESSING("92", "處理中"),
    /** 處理中(已接收) — 僅超商 B2C */
    PROCESSING_RECEIVED("98", "處理中(已接收)"),
    /** 待出貨 */
    WAITING_SHIPMENT("21", "待出貨"),
    /** 物流驗收 — 僅超商 */
    LOGISTICS_INSPECTION("22", "物流驗收"),
    /** 配送中 */
    IN_TRANSIT("31", "配送中"),
    /** 待取貨 — 僅超商 */
    WAITING_PICKUP("32", "待取貨"),
    /** 異常訂單 */
    ABNORMAL("33", "異常訂單"),
    /** 已取貨 — ★ 終態，觸發訂單 COMPLETED */
    PICKED_UP("11", "已取貨"),
    /** 已取消 */
    CANCELLED("41", "已取消"),
    /** 賠償訂單 */
    COMPENSATION("43", "賠償訂單"),
    /** 包裹遺失 */
    LOST("44", "包裹遺失"),
    /** 包裹拋棄 — 僅超商 C2C */
    ABANDONED("46", "包裹拋棄"),
    /** 一般退貨 */
    RETURN_NORMAL("51", "一般退貨"),
    /** 買家未取 — 超過取貨期限 */
    BUYER_NOT_PICKED("52", "買家未取"),
    /** 廠退 — 退回物流中心/集貨所 */
    FACTORY_RETURN("53", "廠退"),
    /** 賣家未取 — 僅超商 C2C */
    SELLER_NOT_PICKED("55", "賣家未取"),
    /** 已轉宅配退回 — 僅超商 C2C */
    CONVERTED_HOME_RETURN("56", "已轉宅配退回"),
    /** 門市關轉 — 需重選門市 */
    STORE_TRANSFER("81", "門市關轉"),
    /** 待轉宅配退回 — 僅超商 C2C */
    PENDING_HOME_RETURN("82", "待轉宅配退回");

    private final String code;
    private final String description;

    ShipStatusCode(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() { return code; }
    public String description() { return description; }

    /**
     * 依狀態碼查找 Enum
     *
     * @param code 狀態碼字串
     * @return 對應的 Enum，找不到回傳 null
     */
    public static ShipStatusCode fromCode(String code) {
        for (ShipStatusCode status : values()) {
            if (status.code.equals(code)) {
                return status;
            }
        }
        return null;
    }
}
