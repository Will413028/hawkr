package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.media.grpc.ServerUploadAndFinalizeResponse;
import io.github.will413028.hawkr.payuni.http.PayUniExchangeContext;
import io.github.will413028.hawkr.payuni.shipping.PayUniShippingEndpoint;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.PrintCvsLabelRequest;
import io.github.will413028.hawkr.payuni.shipping.request.ScheduleHomeLabelRequest;
import io.github.will413028.hawkr.payuni.shipping.response.PrintCvsLabelDirectResponse;
// ScheduleHomeLabelResponse 不再使用（PayUni 回傳 HTML，改用 scheduleHomeLabelHtml）
import io.github.will413028.hawkr.shipping.audit.ShippingPayUniAuditLogger;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * 出貨標籤管理（超商圖片 / 宅配 PDF 共用）
 * 負責呼叫 PayUni → 下載標籤 → 上傳 OSS → 解除引用
 */
@Component
public class ShipmentLabelService {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentLabelService.class);
    private static final String REF_TYPE_SHIPPING = AssetReferenceType.SHIPPING.value();

    private final MediaGrpcClient mediaGrpcClient;
    private final ShippingClient shippingClient;
    private final ShippingPayUniAuditLogger auditLogger;
    private final OkHttpClient httpClient;

    public ShipmentLabelService(MediaGrpcClient mediaGrpcClient, ShippingClient shippingClient,
                                ShippingPayUniAuditLogger auditLogger) {
        this.mediaGrpcClient = mediaGrpcClient;
        this.shippingClient = shippingClient;
        this.auditLogger = auditLogger;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .build();
    }

    /**
     * 預約宅配取號 + 下載 PDF + 上傳至 OSS
     *
     * @param merId        商家編號
     * @param shipTradeNo  PayUni 物流序號
     * @param shipmentNo   內部物流單號（用於 PDF 檔名）
     * @param goodsType    溫層代碼（1/2/3）
     * @param lgsType      物流類型（HOME）
     * @param shipType     出貨類型（2=宅配）
     * @param shipDate     出貨日期（yyyyMMdd）
     * @param deliveryDate 到貨日期（yyyyMMdd）
     * @param spec         規格代碼（1/2/3/4）
     * @return 上傳結果（assetId, objectKey）
     */
    public LabelUploadResult scheduleAndUploadLabel(String merId, String shipTradeNo, String shipmentNo,
                                                     String goodsType, String lgsType, String shipType,
                                                     String shipDate, String deliveryDate, String spec) {
        // 1. 呼叫 PayUni 預約宅配取號（PayUni 回傳 HTML form，非 JSON）
        String html = shippingClient.scheduleHomeLabelHtml(
                merId, new ScheduleHomeLabelRequest(shipTradeNo, goodsType, lgsType, shipType,
                        shipDate, deliveryDate, spec, null, null));

        if (html == null || html.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_SCHEDULE_EMPTY);
        }

        // 1.5 檢查是否為錯誤回應（window.alert）
        if (html.contains("window.alert")) {
            String errorMsg = html.replaceAll(".*window\\.alert\\(\"(.*)\"\\).*", "$1");
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_SCHEDULE_FAILED, errorMsg);
        }

        // 2. 解析 HTML form → 二次 POST 下載 PDF
        byte[] pdfBytes = downloadPdfFromHtmlForm(html);

        log.info("ShipmentLabelService.scheduleLabel.success",
                "預約取號+PDF下載成功 size=" + pdfBytes.length, "shipTradeNo", shipTradeNo);

        // 3. 上傳至 OSS（透過 Media Service）
        String filename = shipmentNo + ".pdf";
        ServerUploadAndFinalizeResponse uploadResp = mediaGrpcClient.serverUploadAndFinalize(
                pdfBytes, filename, "application/pdf", REF_TYPE_SHIPPING);

        log.info("ShipmentLabelService.labelUpload.success",
                "PDF 標籤上傳成功", "objectKey", uploadResp.getObjectKey());

        return new LabelUploadResult(
                ProtoConversionUtils.parseLongRequired(uploadResp.getAssetId(), "assetId"),
                uploadResp.getObjectKey()
        );
    }

    /**
     * 解除 PDF 引用（配送完成後呼叫）。
     *
     * @return 是否確實解除。呼叫端必須據此決定要不要清空 shipments 的 PDF 欄位——
     *         解除失敗卻清了欄位，引用會留在 media-service 裡，那個物件就永遠不會
     *         被 GC 回收（洩漏）；反過來解除成功卻沒清欄位，欄位會在 7 天後變成
     *         指向不存在物件的死指標（#2288 就是這個形狀）。兩者必須同進退。
     */
    public boolean unlinkLabel(Long shipmentId, Long labelAssetId) {
        if (labelAssetId == null || labelAssetId <= 0) {
            return false;
        }
        try {
            mediaGrpcClient.unlinkImages(REF_TYPE_SHIPPING,
                    String.valueOf(shipmentId),
                    List.of(String.valueOf(labelAssetId)));
            log.info("ShipmentLabelService.unlinkLabel.success",
                    "PDF 引用已解除", "shipmentId", String.valueOf(shipmentId));
            return true;
        } catch (Exception e) {
            log.error("ShipmentLabelService.unlinkLabel.error",
                    "解除 PDF 引用失敗", e);
            return false;
        }
    }

    /**
     * 觸發超商列印（僅呼叫 PayUni API，不下載圖片）
     * 出貨便代碼由 Webhook（ApiType=Print）非同步回填
     */
    public void triggerCvsPrint(String merId, String shipTradeNo,
                                 String goodsType, String lgsType,
                                 String shipType, String shipDate) {
        shippingClient.triggerCvsPrint(
                merId, new PrintCvsLabelRequest(shipTradeNo, goodsType, lgsType, shipType, shipDate));
        log.info("ShipmentLabelService.triggerCvsPrint.success",
                "已觸發 PayUni 超商列印", "shipTradeNo", shipTradeNo);
    }

    /**
     * 批次觸發超商列印（多個 ShipTradeNo 合併為逗號分隔）
     * PayUni 要求同批次必須相同 GoodsType/LgsType/ShipType/ShipDate
     */
    public void triggerBatchCvsPrint(String merId, String batchShipTradeNos,
                                       String goodsType, String lgsType,
                                       String shipType, String shipDate) {
        shippingClient.triggerCvsPrint(
                merId, new PrintCvsLabelRequest(batchShipTradeNos, goodsType, lgsType, shipType, shipDate));
        log.info("ShipmentLabelService.triggerBatchCvsPrint.success",
                "已觸發 PayUni 批次超商列印", "shipTradeNos", batchShipTradeNos);
    }

    /**
     * 超商出貨單列印 + 下載標籤圖片 + 上傳至 OSS（已棄用，保留供參考）
     *
     * @param merId       商家編號
     * @param shipmentNo  內部物流單號（用於圖片檔名）
     * @param shipTradeNo PayUni 物流序號
     * @param goodsType   溫層代碼
     * @param lgsType     通路類型（C2C）
     * @param shipType    物流商類型（1=7-ELEVEN）
     * @param shipDate    出貨日期（YYYYMMDD）
     * @return 上傳結果（assetId, objectKey）
     */
    public LabelUploadResult printAndUploadCvsLabel(String merId, String shipmentNo,
                                                     String shipTradeNo, String goodsType,
                                                     String lgsType, String shipType, String shipDate) {
        // 1. 呼叫 PayUni print_label API → 取得標籤圖片 URL
        PrintCvsLabelDirectResponse labelResp = shippingClient.printCvsLabel(
                merId, new PrintCvsLabelRequest(shipTradeNo, goodsType, lgsType, shipType, shipDate));

        String imageUrl = labelResp.imageUrl();
        log.info("ShipmentLabelService.printCvsLabel.success",
                "超商標籤列印成功", "shipTradeNo", shipTradeNo);

        // 2. 下載標籤圖片 bytes
        byte[] imageBytes = downloadBytes(imageUrl);

        // 3. 上傳至 OSS（透過 Media Service）
        String filename = shipmentNo + ".png";
        ServerUploadAndFinalizeResponse uploadResp = mediaGrpcClient.serverUploadAndFinalize(
                imageBytes, filename, "image/png", REF_TYPE_SHIPPING);

        log.info("ShipmentLabelService.cvsLabelUpload.success",
                "超商標籤圖片上傳成功", "objectKey", uploadResp.getObjectKey());

        return new LabelUploadResult(
                ProtoConversionUtils.parseLongRequired(uploadResp.getAssetId(), "assetId"),
                uploadResp.getObjectKey()
        );
    }

    /**
     * 解析 PayUni 回傳的 HTML form，提取參數後 POST cat_file_download 下載 PDF
     *
     * PayUni 宅配取號 API 回傳格式：
     * <form name='catDownload' action='https://.../cat_file_download'>
     *   <input type='hidden' name='MerID' value='...' >
     *   <input type='hidden' name='CustomerId' value='...' >
     *   <input type='hidden' name='CustomerToken' value='...' >
     *   <input type='hidden' name='FileNo' value='...' >
     *   <input type='hidden' name='DownloadFileName' value='...' >
     * </form>
     */
    private byte[] downloadPdfFromHtmlForm(String html) {
        // 提取 form action URL
        String actionUrl = extractAttribute(html, "action");
        if (actionUrl == null || actionUrl.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_FORM_ACTION_MISSING);
        }

        // 提取所有 hidden input 參數（同時保留一份供稽核落庫）
        okhttp3.FormBody.Builder formBuilder = new okhttp3.FormBody.Builder();
        Map<String, String> formParams = new LinkedHashMap<>();
        java.util.regex.Pattern inputPattern = java.util.regex.Pattern.compile(
                "name='([^']+)'\\s+value='([^']*)'");
        java.util.regex.Matcher matcher = inputPattern.matcher(html);
        int paramCount = 0;
        while (matcher.find()) {
            formBuilder.add(matcher.group(1), matcher.group(2));
            formParams.put(matcher.group(1), matcher.group(2));
            paramCount++;
        }

        if (paramCount == 0) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_FORM_PARAMS_MISSING);
        }

        log.info("ShipmentLabelService.downloadPdf.post",
                "二次 POST 下載 PDF，actionUrl=" + actionUrl + " params=" + paramCount);

        // POST form 下載 PDF
        Request request = new Request.Builder()
                .url(actionUrl)
                .post(formBuilder.build())
                .build();

        // §10.4：此二次 POST 不經 PayUniHttpClient，onExchange hook 覆蓋不到，手動補一筆稽核。
        // CustomerToken 由 helper 遮罩；PDF 二進位只記 metadata（response_raw=null）；失敗路徑也記。
        long t0 = System.currentTimeMillis();
        PayUniExchangeContext auditCtx = new PayUniExchangeContext(
                formParams.get("MerID"), PayUniShippingEndpoint.PATH_CAT_FILE_DOWNLOAD, formParams);
        try (Response response = httpClient.newCall(request).execute()) {
            auditCtx.setHttpStatus(response.code());
            if (!response.isSuccessful() || response.body() == null) {
                auditCtx.setErrorCode("HTTP_ERROR");
                auditCtx.setErrorMessage("PDF 下載失敗 HTTP " + response.code());
                log.error("ShipmentLabelService.downloadPdf.httpError",
                        "PDF 下載失敗 HTTP " + response.code());
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_PDF_DOWNLOAD_FAILED);
            }
            byte[] bytes = response.body().bytes();
            if (bytes.length < 100) {
                // PDF 至少數 KB，太小可能是錯誤回應（文字，非二進位 → 保留原文供稽核）
                String bodyStr = new String(bytes);
                auditCtx.setErrorCode("INVALID_RESPONSE");
                auditCtx.setErrorMessage("PDF 下載回應太小，可能為錯誤");
                auditCtx.setRawResponseBody(bodyStr);
                log.error("ShipmentLabelService.downloadPdf.invalidResponse",
                        "PDF 下載回應太小，可能為錯誤: " + bodyStr);
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_PDF_RESPONSE_INVALID);
            }
            // 成功：二進位 PDF 不落內容，僅記 metadata
            auditCtx.setOuterMessage("(PDF " + bytes.length + " bytes, application/pdf)");
            return bytes;
        } catch (IOException e) {
            auditCtx.setErrorCode("IO_EXCEPTION");
            auditCtx.setErrorMessage(e.getMessage());
            auditCtx.setHttpStatus(0);
            log.error("ShipmentLabelService.downloadPdf.ioError",
                    "PDF 下載失敗", e);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_PDF_DOWNLOAD_FAILED);
        } finally {
            auditCtx.setDurationMs(System.currentTimeMillis() - t0);
            auditLogger.recordOutbound(auditCtx);
        }
    }

    /**
     * 從 HTML 中提取指定屬性值（如 action='...'）
     */
    private String extractAttribute(String html, String attr) {
        java.util.regex.Pattern pattern = java.util.regex.Pattern.compile(
                attr + "='([^']+)'");
        java.util.regex.Matcher matcher = pattern.matcher(html);
        return matcher.find() ? matcher.group(1) : null;
    }

    private byte[] downloadBytes(String url) {
        Request request = new Request.Builder().url(url).build();
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful() || response.body() == null) {
                log.error("ShipmentLabelService.downloadBytes.httpError",
                        "檔案下載失敗 HTTP " + response.code(), "url", url);
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_FILE_DOWNLOAD_FAILED);
            }
            return response.body().bytes();
        } catch (IOException e) {
            log.error("ShipmentLabelService.downloadBytes.ioError",
                    "檔案下載失敗 url=" + url, e);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_FILE_DOWNLOAD_FAILED);
        }
    }

    public record LabelUploadResult(Long assetId, String objectKey) {
    }
}
