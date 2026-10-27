package io.github.will413028.hawkr.payuni.http;

import java.util.Map;

/**
 * PayUni API 通用回應封裝
 *
 * @param status        回應狀態碼（SUCCESS 或錯誤碼）
 * @param message       回應訊息
 * @param decryptedData 解密後的業務資料（可能為 null）
 */
public record PayUniApiResponse(
        String status,
        String message,
        Map<String, String> decryptedData
) {

    private static final String FIELD_TRADE_STATUS    = "TradeStatus";
    private static final String FIELD_TRADE_NO        = "TradeNo";
    private static final String FIELD_MER_TRADE_NO    = "MerTradeNo";
    private static final String FIELD_TRADE_AMT       = "TradeAmt";
    private static final String FIELD_BANK_TYPE       = "BankType";
    private static final String FIELD_PAY_NO          = "PayNo";
    private static final String FIELD_EXPIRE_DATE     = "ExpireDate";
    private static final String FIELD_CLOSE_STATUS    = "CloseStatus";
    private static final String FIELD_MESSAGE         = "Message";

    /** 判斷請求是否成功 */
    public boolean isSuccess() {
        return "SUCCESS".equalsIgnoreCase(status);
    }

    /** 取得解密資料中的指定欄位（純取值，不做 fallback） */
    public String getData(String key) {
        return decryptedData != null ? decryptedData.get(key) : null;
    }

    // ===== 業務 getter（同時支援 v2.0 Result[0][...] 與 v1.x 頂層格式）=====

    /**
     * 取得 MerTradeNo（系統交易編號）
     * 同時支援 v2.0 Result 陣列與 v1.x 頂層欄位
     */
    public String getMerTradeNo() {
        return getDualFormat(FIELD_MER_TRADE_NO);
    }

    /**
     * 取得 TradeNo（PayUni UNi 序號）
     * 同時支援 v2.0 Result 陣列與 v1.x 頂層欄位
     */
    public String getProviderTradeNo() {
        return getDualFormat(FIELD_TRADE_NO);
    }

    /**
     * 取得 TradeStatus（交易狀態碼）
     *
     * 同時支援 PayUni 兩種回應格式：
     * - v1.x：頂層 TradeStatus（建單回應 / Apple Pay / Google Pay 即時付款）
     * - v2.0：陣列鍵 Result[0][TradeStatus]（交易查詢 API §2.4）
     */
    public String getTradeStatus() {
        return getDualFormat(FIELD_TRADE_STATUS);
    }

    /**
     * 取得交易金額（Long parse）
     */
    public Long getTradeAmount() {
        String amt = getDualFormat(FIELD_TRADE_AMT);
        if (amt == null || amt.isEmpty()) return null;
        try {
            return Long.parseLong(amt);
        } catch (NumberFormatException e) {
            return null;
        }
    }

    /**
     * 取得 ATM 銀行代碼
     */
    public String getAtmBankCode() {
        return getDualFormat(FIELD_BANK_TYPE);
    }

    /**
     * 取得 ATM 虛擬帳號
     */
    public String getAtmVirtualAccount() {
        return getDualFormat(FIELD_PAY_NO);
    }

    /**
     * 取得 ATM 繳費期限
     */
    public String getAtmExpireDate() {
        return getDualFormat(FIELD_EXPIRE_DATE);
    }

    /**
     * 取得信用卡請款狀態（查詢 API 回傳）
     *
     * v2.0 查詢 API 回傳 Result 陣列，key 為 Result[0][CloseStatus]。
     * 1=請款申請中, 2=請款成功, 3=請款取消, 7=請款處理中, 9=未申請
     */
    public String getCloseStatus() {
        return getDualFormat(FIELD_CLOSE_STATUS);
    }

    /**
     * 取得錯誤訊息
     * 優先讀解密 data 內的 Message，若無則回外層 message
     */
    public String getErrorMessage() {
        if (decryptedData != null) {
            String msg = decryptedData.get(FIELD_MESSAGE);
            if (msg != null && !msg.isEmpty()) {
                return msg;
            }
        }
        return message;
    }

    /**
     * 雙格式取值：優先讀 Result[0][field]（v2.0），若無則讀頂層 field（v1.x）
     */
    private String getDualFormat(String field) {
        if (decryptedData == null) return null;
        String v = decryptedData.get("Result[0][" + field + "]");
        if (v == null || v.isEmpty()) {
            v = decryptedData.get(field);
        }
        return (v == null || v.isEmpty()) ? null : v;
    }

    // ===== 撥款查詢（grant_query）Result 陣列逐筆取值（§5.6 去重硬化 B）=====
    // 解密後 decryptedData 的 key 形如 Result[0][TradeNo] / Result[1][Type] ...（PayUniHttpClient
    // parseResponse 解 URL query string；欄位名依代理商專用 API 總覽 §6.4 實名，不臆造）。

    /**
     * grant_query 回傳的 Result 陣列筆數（以 Result[i][TradeNo] / Result[i][Type] 是否存在為界）。
     */
    public int getGrantQueryResultCount() {
        if (decryptedData == null) return 0;
        int i = 0;
        while (decryptedData.containsKey("Result[" + i + "][TradeNo]")
                || decryptedData.containsKey("Result[" + i + "][Type]")) {
            i++;
        }
        return i;
    }

    /** 第 i 筆 Result 的 UNi 序號（TradeNo）。 */
    public String getGrantQueryTradeNo(int index) {
        return grantQueryField(index, "TradeNo");
    }

    /** 第 i 筆 Result 的類別（Type，入帳/出帳；PayUni 未明列代碼，僅供稽核 log）。 */
    public String getGrantQueryType(int index) {
        return grantQueryField(index, "Type");
    }

    /** 第 i 筆 Result 的入帳金額（FundsAmt，扣費後淨額）。 */
    public String getGrantQueryFundsAmt(int index) {
        return grantQueryField(index, "FundsAmt");
    }

    /** 第 i 筆 Result 的執行金額（ExecuteAmt）。 */
    public String getGrantQueryExecuteAmt(int index) {
        return grantQueryField(index, "ExecuteAmt");
    }

    private String grantQueryField(int index, String field) {
        if (decryptedData == null) return null;
        String v = decryptedData.get("Result[" + index + "][" + field + "]");
        return (v == null || v.isEmpty()) ? null : v;
    }

    /** grant_query 分頁總頁數（PageInfo[Total]）；缺省或無法解析時視為 1（單頁）。 */
    public int getGrantQueryPageTotal() {
        if (decryptedData == null) return 1;
        String v = decryptedData.get("PageInfo[Total]");
        if (v == null || v.isBlank()) return 1;
        try {
            int total = Integer.parseInt(v.trim());
            return total > 0 ? total : 1;
        } catch (NumberFormatException e) {
            return 1;
        }
    }
}
