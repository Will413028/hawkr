package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.application.dto.PaymentDetailResponse;
import io.github.will413028.hawkr.payuni.payment.model.UppFormData;
import com.fasterxml.jackson.core.JsonProcessingException;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.Optional;

/**
 * 付款頁面服務
 * <p>
 * 負責組裝付款相關的 URL 與 HTML 頁面：
 * - redirectUrl：WebView 開啟 URL（帶不可猜 opaque id，canonical 表單存 Redis）
 * - 付款結果回調頁面：含狀態映射、Deep Link 導向的完整 HTML
 * <p>
 * Gateway 僅透傳這些結果，不做任何業務邏輯。
 */
@Service
public class PaymentPageService {

    private static final ObsLogger log = ObsLogger.forClass(PaymentPageService.class);

    private final PaymentRedirectFormStore redirectFormStore;

    @Value("${gateway.url:}")
    private String gatewayUrl;

    public PaymentPageService(PaymentRedirectFormStore redirectFormStore) {
        this.redirectFormStore = redirectFormStore;
    }

    /**
     * 組裝 redirectUrl（canonical 表單存 Redis 換 opaque id → GATEWAY_URL/payment/redirect?id=xxx）
     *
     * @param uppFormData UPP 表單資料
     * @return 完整的 redirect URL，若 GATEWAY_URL 未設定則回傳空字串
     */
    public String buildRedirectUrl(UppFormData uppFormData) {
        if (gatewayUrl == null || gatewayUrl.isEmpty() || uppFormData == null) {
            return "";
        }

        try {
            String id = redirectFormStore.store(new PaymentRedirectFormStore.RedirectForm(
                    uppFormData.actionUrl(),
                    uppFormData.merId(),
                    uppFormData.version(),
                    uppFormData.encryptInfo(),
                    uppFormData.hashInfo(),
                    uppFormData.isPlatform()));
            return gatewayUrl + "/payment/redirect?id=" + id;
        } catch (JsonProcessingException e) {
            log.error("PaymentPageService.buildRedirectUrl.error", "JSON 序列化失敗: " + e.getMessage(), e);
            return "";
        }
    }

    /**
     * 依 opaque id 一次性取回付款中轉頁 canonical 表單（供 Gateway redirect handler 使用）。
     *
     * @param redirectId opaque redirect id
     * @return canonical 表單，若查無 / 過期 / 已用則為 empty
     */
    public Optional<PaymentRedirectFormStore.RedirectForm> consumeRedirectForm(String redirectId) {
        return redirectFormStore.consume(redirectId);
    }

    /**
     * 根據交易詳情產生付款結果 HTML 頁面
     *
     * @param detail 交易詳情（可為 null）
     * @return 完整 HTML 頁面
     */
    public String renderPaymentReturnPage(PaymentDetailResponse detail) {
        if (detail == null) {
            return renderErrorHtml("交易不存在");
        }

        PaymentResultDisplay display = mapDisplayStatus(detail.status());
        String tradeNo = detail.tradeNo();
        String amount = String.valueOf(detail.amount());

        return renderStatusHtml(display, tradeNo, amount);
    }

    /**
     * 產生錯誤頁面 HTML
     */
    public String renderErrorHtml(String errorMsg) {
        return renderStatusHtml(PaymentResultDisplay.ERROR, "", "");
    }

    // ==================== 私有方法 ====================

    /**
     * 將交易狀態映射為 UI 顯示狀態
     */
    private PaymentResultDisplay mapDisplayStatus(PaymentTransactionStatus status) {
        return switch (status) {
            case PAID -> PaymentResultDisplay.SUCCESS;
            case FAILED, EXPIRED, CONFLICT, CLOSED, CLOSED_BY_SWITCH -> PaymentResultDisplay.FAILED;
            case INIT -> PaymentResultDisplay.PENDING;
        };
    }

    /**
     * 產生付款結果 HTML 頁面（含 Deep Link 導向）
     */
    private String renderStatusHtml(PaymentResultDisplay display, String tradeNo, String amount) {
        String icon, title, subtitle, bgColor;

        switch (display) {
            case SUCCESS -> {
                icon = "\u2705";
                title = "付款成功";
                subtitle = "感謝您的購買！";
                bgColor = "#f0fdf4";
            }
            case FAILED -> {
                icon = "\u274C";
                title = "付款失敗";
                subtitle = "請重新嘗試或選擇其他付款方式";
                bgColor = "#fef2f2";
            }
            case ERROR -> {
                icon = "\u26A0\uFE0F";
                title = "系統異常";
                subtitle = "請稍後重試";
                bgColor = "#fffbeb";
            }
            default -> {
                icon = "\u23F3";
                title = "付款處理中";
                subtitle = "請稍候，系統正在確認付款結果";
                bgColor = "#eff6ff";
            }
        }

        // Deep Link（付款目前僅限 BUYER）
        String deepLink = "hawkrbuyer://payment/result?tradeNo=" + escapeHtml(tradeNo) + "&status=" + display.code();

        // 金額顯示
        String amountHtml = "";
        if (amount != null && !amount.isEmpty()) {
            amountHtml = "<p style=\"font-size:18px;color:#374151;margin:8px 0;\">金額：NT$ " + escapeHtml(amount) + "</p>";
        }

        // 交易編號顯示
        String tradeNoHtml = "";
        if (tradeNo != null && !tradeNo.isEmpty()) {
            tradeNoHtml = "<p style=\"font-size:14px;color:#6b7280;margin:4px 0;\">交易編號：" + escapeHtml(tradeNo) + "</p>";
        }

        return "<!DOCTYPE html>\n"
                + "<html lang=\"zh-TW\">\n"
                + "<head>\n"
                + "    <meta charset=\"UTF-8\">\n"
                + "    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"
                + "    <title>" + escapeHtml(title) + " - Hawkr</title>\n"
                + "    <style>\n"
                + "        * { margin: 0; padding: 0; box-sizing: border-box; }\n"
                + "        body {\n"
                + "            font-family: -apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif;\n"
                + "            display: flex; justify-content: center; align-items: center;\n"
                + "            min-height: 100vh; background: " + bgColor + ";\n"
                + "        }\n"
                + "        .container {\n"
                + "            text-align: center; padding: 40px 24px;\n"
                + "            background: white; border-radius: 16px;\n"
                + "            box-shadow: 0 4px 24px rgba(0,0,0,0.08);\n"
                + "            max-width: 400px; width: 90%;\n"
                + "        }\n"
                + "        .icon { font-size: 64px; margin-bottom: 16px; }\n"
                + "        .title { font-size: 24px; font-weight: 600; color: #1f2937; margin-bottom: 8px; }\n"
                + "        .btn {\n"
                + "            display: inline-block; margin-top: 24px; padding: 14px 32px;\n"
                + "            background: #10b981; color: white; border: none; border-radius: 12px;\n"
                + "            font-size: 16px; font-weight: 500; text-decoration: none; cursor: pointer;\n"
                + "        }\n"
                + "        .btn:active { background: #059669; }\n"
                + "        .countdown { font-size: 13px; color: #9ca3af; margin-top: 16px; }\n"
                + "    </style>\n"
                + "</head>\n"
                + "<body>\n"
                + "    <div class=\"container\">\n"
                + "        <div class=\"icon\">" + icon + "</div>\n"
                + "        <div class=\"title\">" + escapeHtml(title) + "</div>\n"
                + "        <p style=\"font-size:15px;color:#6b7280;margin-bottom:12px;\">" + escapeHtml(subtitle) + "</p>\n"
                + "        " + amountHtml + "\n"
                + "        " + tradeNoHtml + "\n"
                + "        <a class=\"btn\" id=\"returnBtn\" href=\"" + deepLink + "\">返回 App</a>\n"
                + "        <p class=\"countdown\" id=\"countdown\">10 秒後自動返回 App...</p>\n"
                + "    </div>\n"
                + "    <script>\n"
                + "        var seconds = 10;\n"
                + "        var deepLink = \"" + escapeJs(deepLink) + "\";\n"
                + "        var timer = setInterval(function() {\n"
                + "            seconds--;\n"
                + "            var el = document.getElementById(\"countdown\");\n"
                + "            if (seconds > 0) {\n"
                + "                el.textContent = seconds + \" 秒後自動返回 App...\";\n"
                + "            } else {\n"
                + "                clearInterval(timer);\n"
                + "                el.textContent = \"正在返回 App...\";\n"
                + "                window.location.href = deepLink;\n"
                + "            }\n"
                + "        }, 1000);\n"
                + "    </script>\n"
                + "</body>\n"
                + "</html>";
    }

    private String escapeHtml(String value) {
        if (value == null) return "";
        return value.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;");
    }

    private String escapeJs(String value) {
        if (value == null) return "";
        return value.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n");
    }
}
