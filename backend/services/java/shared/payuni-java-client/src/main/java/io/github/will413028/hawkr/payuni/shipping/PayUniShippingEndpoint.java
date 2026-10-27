package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 物流 API 端點與版本常數
 *
 * 端點路徑與版本參照官方 API 文檔：
 * - 超商: /api/logistics/trade (v1.3)
 * - 宅配: /api/home_delivery/trade (v1.2)
 * - 產宅配編號並下載託運單 PDF: /api/home_delivery/get_obt_number_pdf (v1.0)
 * - 物流查詢: /api/logistics/query (v1.1)
 */
public final class PayUniShippingEndpoint {

    private PayUniShippingEndpoint() {}

    /** 超商物流單建立（7-ELEVEN C2C/B2C） v1.3 */
    public static final String PATH_CVS_SHIPMENT = "/api/logistics/trade";
    public static final String VERSION_CVS_SHIPMENT = "1.3";

    /** 宅配單建立（黑貓宅急便，背景 API） v1.2 */
    public static final String PATH_HOME_SHIPMENT = "/api/home_delivery/trade";
    public static final String VERSION_HOME_SHIPMENT = "1.2";

    /** 產宅配編號並下載託運單 PDF v1.0 */
    public static final String PATH_HOME_LABEL = "/api/home_delivery/get_obt_number_pdf";
    public static final String VERSION_HOME_LABEL = "1.0";

    /** 超商出貨單列印 v1.0（後端代理呼叫，回傳 HTML） */
    public static final String PATH_PRINT_CVS_LABEL = "/api/logistics/print_label";
    public static final String VERSION_PRINT_CVS_LABEL = "1.0";

    /** 呼叫黑貓預約取件 v1.0 */
    public static final String PATH_CALL_TCAT = "/api/home_delivery/call_cat";
    public static final String VERSION_CALL_TCAT = "1.0";

    /** 物流單查詢 v1.1 */
    public static final String PATH_QUERY_SHIPMENT = "/api/logistics/query";
    public static final String VERSION_QUERY_SHIPMENT = "1.1";

    /** 超商物流建店 v1.0 */
    public static final String PATH_CVS_ONBOARD = "/api/logistics/setting";
    public static final String VERSION_CVS_ONBOARD = "1.0";

    /** 宅配物流建店 v1.0 */
    public static final String PATH_HOME_ONBOARD = "/api/home_delivery/setting";
    public static final String VERSION_HOME_ONBOARD = "1.0";

    /** 超商門市地圖（前景 API） v1.1 */
    public static final String PATH_STORE_MAP = "/api/logistics/ship_map";
    public static final String VERSION_STORE_MAP = "1.1";

    /** 宅配退貨單建立（黑貓退貨，背景 API） v1.0 */
    public static final String PATH_HOME_REFUND = "/api/home_delivery/refund";
    public static final String VERSION_HOME_REFUND = "1.0";

    /** 託運單 PDF 二次下載（宅配取號 HTML form 的 action 目標；非我方組 URL，僅供稽核 api_path 標記） */
    public static final String PATH_CAT_FILE_DOWNLOAD = "cat_file_download";
}
