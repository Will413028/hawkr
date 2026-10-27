package io.github.will413028.hawkr.payuni.shipping;

import io.github.will413028.hawkr.payuni.http.PayUniApiException;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.shipping.request.*;
import io.github.will413028.hawkr.payuni.shipping.response.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * PayUni 物流 API 客戶端
 * <p>
 * 提供 4 個物流 API：超商物流單建立、宅配單建立、預約宅配取號、物流單查詢。
 * 代理商模式下，透過 PayUniHttpClient 自動帶 IsPlatForm=1。
 */
public class ShippingClient {

    private static final Logger log = LoggerFactory.getLogger(ShippingClient.class);
    private static final Pattern IMG_SRC_PATTERN = Pattern.compile("<img[^>]+src=[\"']([^\"']+)[\"']", Pattern.CASE_INSENSITIVE);
    private static final Pattern ALERT_PATTERN = Pattern.compile("window\\.alert\\([\"']([^\"']*)[\"']\\)", Pattern.CASE_INSENSITIVE);

    private final PayUniHttpClient httpClient;
    private final String notifyUrl;

    public ShippingClient(PayUniHttpClient httpClient) {
        this(httpClient, null);
    }

    public ShippingClient(PayUniHttpClient httpClient, String notifyUrl) {
        this.httpClient = httpClient;
        this.notifyUrl = notifyUrl;
    }

    /**
     * 建立超商物流單 v1.3
     */
    public CreateCvsShipmentResponse createCvsShipment(String merId, CreateCvsShipmentRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("MerTradeNo", req.merTradeNo());
        params.put("ShipType", req.shipType());
        params.put("LgsType", req.lgsType());
        putIfNotNull(params, "GoodsType", req.goodsType());
        putIfNotNull(params, "ServiceType", req.serviceType());
        putIfNotNull(params, "SenderName", req.senderName());
        putIfNotNull(params, "SenderMobile", toLocalPhone(req.senderPhone()));
        params.put("Consignee", req.receiverName());
        params.put("ConsigneeMobile", toLocalPhone(req.receiverPhone()));
        params.put("StoreID", req.storeId());
        params.put("TradeAmt", req.tradeAmt());
        putIfNotNull(params, "ProdDesc", req.goodsName());
        putIfNotNull(params, "RefundStoreID", req.refundStoreId());
        putIfNotNull(params, "NotifyURL", notifyUrl);

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_CVS_SHIPMENT, params, PayUniShippingEndpoint.VERSION_CVS_SHIPMENT);
        return CreateCvsShipmentResponse.from(resp);
    }

    /**
     * 建立宅配單 v1.2（背景）
     */
    public CreateHomeShipmentResponse createHomeShipment(String merId, CreateHomeShipmentRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("MerTradeNo", req.merTradeNo());
        putIfNotNull(params, "GoodsType", req.goodsType());
        params.put("LgsType", "HOME");
        params.put("ShipType", "2");
        params.put("ServiceType", "3");
        // 寄件人資訊（PayUni v1.2 必填）
        putIfNotNull(params, "SenderName", req.senderName());
        putIfNotNull(params, "SenderMobile", toLocalPhone(req.senderPhone()));
        putIfNotNull(params, "SenderZipCode", req.senderZipCode());
        putIfNotNull(params, "SenderAddress", req.senderAddress());
        // 收件人資訊
        params.put("Consignee", req.receiverName());
        params.put("ConsigneeMobile", toLocalPhone(req.receiverPhone()));
        params.put("ConsigneeAddress", req.receiverAddress());
        putIfNotNull(params, "DeliveryTimeTag", req.deliveryTimeTag());
        params.put("TradeAmt", req.tradeAmt());
        putIfNotNull(params, "ProdDesc", req.goodsName());
        putIfNotNull(params, "NotifyURL", notifyUrl);

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_HOME_SHIPMENT, params, PayUniShippingEndpoint.VERSION_HOME_SHIPMENT);
        return CreateHomeShipmentResponse.from(resp);
    }

    /**
     * 建立宅配退貨單 v1.0（黑貓退貨）
     *
     * 對應 PayUni API: /api/home_delivery/refund
     * 黑貓司機將於 ShipDate 至退貨人地址收取退件。
     */
    public CreateHomeRefundResponse createHomeRefund(String merId, CreateHomeRefundRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        putIfNotNull(params, "ShipTradeNo", req.shipTradeNo());
        putIfNotNull(params, "MerTradeNo", req.merTradeNo());
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", "HOME");
        params.put("ShipType", "2");
        params.put("ServiceType", "3");
        params.put("DeliveryTimeTag", req.deliveryTimeTag());
        // 收件人（賣家退貨聯絡人）
        params.put("Consignee", req.consigneeName());
        params.put("ConsigneeMobile", toLocalPhone(req.consigneePhone()));
        params.put("ConsigneeAddress", req.consigneeAddress());
        // 退貨人（買家）
        params.put("Consignor", req.consignorName());
        params.put("ConsignorMobile", toLocalPhone(req.consignorPhone()));
        params.put("ConsignorAddress", req.consignorAddress());
        // 退貨專屬參數
        params.put("Spec", req.spec());
        params.put("ShipDate", req.shipDate());
        params.put("ProdDesc", req.goodsName());
        putIfNotNull(params, "Memo", req.memo());
        putIfNotNull(params, "NotifyURL", notifyUrl);

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_HOME_REFUND, params, PayUniShippingEndpoint.VERSION_HOME_REFUND);
        return CreateHomeRefundResponse.from(resp);
    }

    /**
     * 預約宅配取號 v1.0 → 回傳含標籤下載 URL 連結
     */
    public ScheduleHomeLabelResponse scheduleHomeLabel(String merId, ScheduleHomeLabelRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ShipTradeNo", req.shipTradeNo());
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        params.put("ShipDate", req.shipDate());
        params.put("DeliveryDate", req.deliveryDate());
        putIfNotNull(params, "Spec", req.spec());
        putIfNotNull(params, "PostType", req.postType());
        putIfNotNull(params, "PrintType", req.printType());

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_HOME_LABEL, params, PayUniShippingEndpoint.VERSION_HOME_LABEL);
        return ScheduleHomeLabelResponse.from(resp);
    }

    /**
     * 預約宅配取號 v1.0 → 回傳 HTML（PayUni 實際回傳 HTML form，非 JSON）
     * 用於二階段 PDF 下載：先取得 form 參數，再 POST cat_file_download 取得 PDF
     */
    public String scheduleHomeLabelHtml(String merId, ScheduleHomeLabelRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ShipTradeNo", req.shipTradeNo());
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        params.put("ShipDate", req.shipDate());
        params.put("DeliveryDate", req.deliveryDate());
        putIfNotNull(params, "Spec", req.spec());
        putIfNotNull(params, "PostType", req.postType());
        putIfNotNull(params, "PrintType", req.printType());

        return httpClient.callApiHtml(
                merId, PayUniShippingEndpoint.PATH_HOME_LABEL, params, PayUniShippingEndpoint.VERSION_HOME_LABEL);
    }

    /**
     * 超商出貨單列印 v1.0（後端代理模式）
     * <p>
     * 後端呼叫 PayUni print_label API → 解析 HTML → 提取標籤圖片 URL。
     * PayUni 回傳 HTML（非 JSON），成功時含 {@code <img src="...">}，處理中時含 {@code window.alert("...LAB02022")}。
     *
     * @throws PayUniApiException 當 PayUni 回傳處理中錯誤或無法解析圖片 URL 時
     */
    public PrintCvsLabelDirectResponse printCvsLabel(String merId, PrintCvsLabelRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ShipTradeNo", req.shipTradeNo());
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        params.put("ShipDate", req.shipDate());
        params.put("LabelMode", "1");

        String html = httpClient.callApiHtml(
                merId, PayUniShippingEndpoint.PATH_PRINT_CVS_LABEL, params, PayUniShippingEndpoint.VERSION_PRINT_CVS_LABEL);

        // 檢查 PayUni 是否回傳處理中錯誤
        Matcher alertMatcher = ALERT_PATTERN.matcher(html);
        if (alertMatcher.find()) {
            String alertMsg = alertMatcher.group(1);
            log.warn("PayUni print_label 回傳處理中: {}", alertMsg);
            throw new PayUniApiException("LABEL_PROCESSING", "標籤尚在處理中: " + alertMsg, 200);
        }

        // 解析 <img src="..."> 提取圖片路徑
        Matcher imgMatcher = IMG_SRC_PATTERN.matcher(html);
        if (!imgMatcher.find()) {
            log.error("PayUni print_label 回傳 HTML 中找不到 <img> 標籤, html={}", html);
            throw new PayUniApiException("LABEL_PARSE_ERROR", "無法從回應中解析標籤圖片", 200);
        }

        String imgSrc = imgMatcher.group(1);
        // 相對路徑拼接 baseUrl
        String imageUrl;
        if (imgSrc.startsWith("http")) {
            imageUrl = imgSrc;
        } else {
            imageUrl = httpClient.getBaseUrl() + imgSrc;
        }

        log.info("PayUni print_label 成功, imageUrl={}", imageUrl);
        return new PrintCvsLabelDirectResponse(imageUrl);
    }

    /**
     * 超商出貨單列印 — 僅觸發（不解析圖片 URL）
     * <p>
     * 適用於簡化列印和批次列印場景：呼叫 PayUni print_label API 後，
     * 出貨便代碼由 Webhook（ApiType=Print）逐筆回填，無需解析 HTML 圖片。
     * ShipTradeNo 可傳入逗號分隔的多筆（最多 50 筆，需相同 GoodsType/LgsType/ShipType）。
     *
     * @throws PayUniApiException 當 PayUni 回傳處理中錯誤時
     */
    public void triggerCvsPrint(String merId, PrintCvsLabelRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ShipTradeNo", req.shipTradeNo());
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        params.put("ShipDate", req.shipDate());
        params.put("LabelMode", "1");

        String html = httpClient.callApiHtml(
                merId, PayUniShippingEndpoint.PATH_PRINT_CVS_LABEL, params, PayUniShippingEndpoint.VERSION_PRINT_CVS_LABEL);

        // 僅檢查 alert（處理中錯誤），不解析 img
        Matcher alertMatcher = ALERT_PATTERN.matcher(html);
        if (alertMatcher.find()) {
            String alertMsg = alertMatcher.group(1);
            log.warn("PayUni print_label 回傳處理中: {}", alertMsg);
            throw new PayUniApiException("LABEL_PROCESSING", "標籤尚在處理中: " + alertMsg, 200);
        }

        log.info("PayUni triggerCvsPrint 成功, shipTradeNo={}", req.shipTradeNo());
    }

    /**
     * 呼叫黑貓預約取件 v1.0
     */
    public CallTcatResponse callTcat(String merId, CallTcatRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ContactName", req.contactName());
        params.put("ContactMobile", toLocalPhone(req.contactMobile()));
        params.put("ContactAddress", req.contactAddress());
        putIfNotNull(params, "NormalQuantity", req.normalQuantity());
        putIfNotNull(params, "ColdQuantity", req.coldQuantity());
        putIfNotNull(params, "FreezeQuantity", req.freezeQuantity());
        putIfNotNull(params, "IsContact", req.isContact());
        putIfNotNull(params, "IsTrolley", req.isTrolley());
        putIfNotNull(params, "Memo", req.memo());

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_CALL_TCAT, params, PayUniShippingEndpoint.VERSION_CALL_TCAT);
        return CallTcatResponse.from(resp);
    }

    /**
     * 物流單查詢 v1.1
     */
    public QueryShipmentResponse queryShipment(String merId, QueryShipmentRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("LgsType", req.lgsType());
        putIfNotNull(params, "ShipTradeNo", req.shipTradeNo());
        putIfNotNull(params, "MerTradeNo", req.merTradeNo());

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_QUERY_SHIPMENT, params, PayUniShippingEndpoint.VERSION_QUERY_SHIPMENT);
        return QueryShipmentResponse.from(resp);
    }

    /**
     * 超商 C2C 建店 v1.0
     */
    public OnboardShippingResponse onboardCvsShipping(String merId, OnboardCvsRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("GoodsType", req.goodsType());
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        putIfNotNull(params, "LabelCompanyName", req.labelCompanyName());
        params.put("ContactName", req.contactName());
        params.put("ContactTel", toLocalPhone(req.contactTel()));
        params.put("ContactMail", req.contactMail());
        params.put("StoreID", req.storeId());
        putIfNotNull(params, "NotifyURL", req.notifyUrl());

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_CVS_ONBOARD, params, PayUniShippingEndpoint.VERSION_CVS_ONBOARD);
        return OnboardShippingResponse.from(resp);
    }

    /**
     * 宅配建店 v1.0
     */
    public OnboardShippingResponse onboardHomeDelivery(String merId, OnboardHomeRequest req) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("LgsType", req.lgsType());
        params.put("ShipType", req.shipType());
        params.put("SenderName", req.senderName());
        params.put("SenderMobile", toLocalPhone(req.senderMobile()));
        params.put("SenderAddress", req.senderAddress());
        params.put("ProductTypeId", req.productTypeId());
        putIfNotNull(params, "NotifyURL", req.notifyUrl());

        PayUniApiResponse resp = httpClient.callApi(
                merId, PayUniShippingEndpoint.PATH_HOME_ONBOARD, params, PayUniShippingEndpoint.VERSION_HOME_ONBOARD);
        return OnboardShippingResponse.from(resp);
    }

    private void putIfNotNull(Map<String, String> map, String key, String value) {
        if (value != null && !value.isEmpty()) {
            map.put(key, value);
        }
    }

    /**
     * 將國際格式電話（+886912345678）轉換為 PayUni 要求的本地格式（0912345678）
     */
    private String toLocalPhone(String phone) {
        if (phone == null || phone.isEmpty()) {
            return phone;
        }
        if (phone.startsWith("+886")) {
            return "0" + phone.substring(4);
        }
        return phone;
    }
}
