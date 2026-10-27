package io.github.will413028.hawkr.payuni.http;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * PayUniApiResponse v1.x / v2.0 雙格式相容測試
 *
 * 對應審計報告 §5 F4 與 P0-A1：
 * PayUni 查詢 API v2.0 把所有欄位包在 Result[0][...] 陣列中，
 * 而 v1.x 建單 / Apple Pay / Google Pay / refund 等回應使用頂層欄位。
 * 同一個 PayUniApiResponse 類別必須能解兩種格式，並保證向後相容。
 */
class PayUniApiResponseDualFormatTest {

    private PayUniApiResponse make(Map<String, String> data) {
        return new PayUniApiResponse("SUCCESS", "ok", data);
    }

    // ===== getTradeStatus =====

    @Test
    @DisplayName("v2.0 Result[0][TradeStatus] 應被讀到")
    void tradeStatus_v2() {
        var r = make(Map.of("Result[0][TradeStatus]", "1"));
        assertEquals("1", r.getTradeStatus());
    }

    @Test
    @DisplayName("v1.x 頂層 TradeStatus 應被讀到（fallback）")
    void tradeStatus_v1() {
        var r = make(Map.of("TradeStatus", "1"));
        assertEquals("1", r.getTradeStatus());
    }

    @Test
    @DisplayName("v2.0 與 v1.x 同時存在時必須優先 v2.0")
    void tradeStatus_v2TakesPrecedence() {
        var r = make(Map.of(
                "Result[0][TradeStatus]", "1",
                "TradeStatus", "0"
        ));
        assertEquals("1", r.getTradeStatus());
    }

    // ===== getTradeNo / getProviderTradeNo =====

    @Test
    @DisplayName("getTradeNo / getProviderTradeNo 雙格式")
    void tradeNo_dualFormat() {
        var v2 = make(Map.of(
                "Result[0][MerTradeNo]", "ORD_001",
                "Result[0][TradeNo]", "UNI_001"
        ));
        assertEquals("ORD_001", v2.getMerTradeNo());
        assertEquals("UNI_001", v2.getProviderTradeNo());

        var v1 = make(Map.of("MerTradeNo", "ORD_001", "TradeNo", "UNI_001"));
        assertEquals("ORD_001", v1.getMerTradeNo());
        assertEquals("UNI_001", v1.getProviderTradeNo());
    }

    // ===== getTradeAmount =====

    @Test
    @DisplayName("getTradeAmount 雙格式 + Long parse")
    void tradeAmount_parsedToLong() {
        assertEquals(500L, make(Map.of("Result[0][TradeAmt]", "500")).getTradeAmount());
        assertEquals(500L, make(Map.of("TradeAmt", "500")).getTradeAmount());
    }

    @Test
    @DisplayName("getTradeAmount 解析失敗回 null（不丟例外）")
    void tradeAmount_parseFailReturnsNull() {
        assertNull(make(Map.of("TradeAmt", "abc")).getTradeAmount());
    }

    // ===== getCloseStatus =====

    @Test
    @DisplayName("getCloseStatus v2.0 / v1.x 雙格式")
    void closeStatus_dualFormat() {
        assertEquals("2", make(Map.of("Result[0][CloseStatus]", "2")).getCloseStatus());
        assertEquals("2", make(Map.of("CloseStatus", "2")).getCloseStatus());
    }

    // ===== ATM 欄位 =====

    @Test
    @DisplayName("ATM 三欄位雙格式")
    void atmFields_dualFormat() {
        var v1 = make(Map.of(
                "BankType", "822",
                "PayNo", "8220100000182918",
                "ExpireDate", "2026-04-09"
        ));
        assertEquals("822", v1.getAtmBankCode());
        assertEquals("8220100000182918", v1.getAtmVirtualAccount());
        assertEquals("2026-04-09", v1.getAtmExpireDate());
    }

    // ===== null safety / 空字串 =====

    @Test
    @DisplayName("decryptedData 為 null 時所有 getter 安全回 null")
    void nullData_allGettersSafe() {
        var r = new PayUniApiResponse("SUCCESS", "ok", null);
        assertNull(r.getTradeStatus());
        assertNull(r.getMerTradeNo());
        assertNull(r.getProviderTradeNo());
        assertNull(r.getTradeAmount());
        assertNull(r.getCloseStatus());
        assertNull(r.getAtmBankCode());
        assertNull(r.getAtmVirtualAccount());
        assertNull(r.getAtmExpireDate());
    }

    @Test
    @DisplayName("空字串視為 null（避免空字串被當有效狀態）")
    void emptyString_treatedAsNull() {
        var r = make(Map.of(
                "Result[0][TradeStatus]", "",
                "TradeStatus", ""
        ));
        assertNull(r.getTradeStatus());
    }

    // ===== getErrorMessage =====

    @Test
    @DisplayName("getErrorMessage 優先讀解密 data Message")
    void errorMessage_dataPrecedence() {
        Map<String, String> data = new HashMap<>();
        data.put("Message", "解密訊息");
        var r = new PayUniApiResponse("FAIL", "外層訊息", data);
        assertEquals("解密訊息", r.getErrorMessage());
    }

    @Test
    @DisplayName("getErrorMessage 解密 data 無 Message 時 fallback 外層 message")
    void errorMessage_fallbackToOuter() {
        var r = new PayUniApiResponse("FAIL", "外層訊息", Map.of());
        assertEquals("外層訊息", r.getErrorMessage());
    }

    // ===== getData 不做 fallback（明確語意）=====

    @Test
    @DisplayName("getData 是純取值，不做雙格式 fallback")
    void getData_noFallback() {
        var r = make(Map.of("Result[0][TradeStatus]", "1"));
        // 直接讀頂層 key 應該回 null
        assertNull(r.getData("TradeStatus"));
        // 讀 Result 陣列 key 才有值
        assertEquals("1", r.getData("Result[0][TradeStatus]"));
    }
}
