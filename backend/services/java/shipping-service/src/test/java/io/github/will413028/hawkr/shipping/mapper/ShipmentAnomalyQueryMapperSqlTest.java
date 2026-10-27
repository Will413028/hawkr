package io.github.will413028.hawkr.shipping.mapper;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@code PAYUNI_LGS_NO_CODE} 偵測述詞的契約釘樁（issue #2789）。
 *
 * <p><b>它證明什麼</b>：Dashboard 推導三段碼缺失時，把<b>空字串也算成缺失</b>
 * —— 三個欄位都必須以 {@code NULLIF(欄位, '') IS NULL} 判定，不得是裸 {@code 欄位 IS NULL}。
 *
 * <p><b>為什麼這條非釘不可</b>：PayUni webhook 缺欄位時寫進 DB 的是<b>空字串不是 NULL</b>
 * （{@code PayUniWebhookEnvelope.payloadField} 對缺欄位回 {@code ""}，
 * {@code ShipmentMapper.markLabelPrintSucceededByWebhook} 的 {@code SET} 直接寫該值、無 NULLIF）。
 * 而 SQL 的 {@code '' IS NULL} <b>恆為 false</b> ⇒ 裸述詞下這類異常在賣家 dashboard
 * <b>一筆都不會出現</b>。它與 {@code ShippingService} 的 webhook Print 路徑耦合：那裡的
 * {@code publishAnomalyDetected} 不是排程、不會重試 ⇒ 推播與 dashboard 兩條路同時失聲。
 *
 * <p><b>它不證明什麼（誠實標註）</b>：這是對 SQL <b>字面</b>的斷言，不是對 PostgreSQL
 * 執行結果的斷言 —— shipping-service 沒有 DB 測試基礎設施（testcontainers 只在
 * cart / aftersales / coupon，且被預設 {@code mvn test} 排除）。它擋得住「有人把 NULLIF 拿掉」
 * 這個回歸，擋不住「NULLIF 的語意其實不是我以為的那樣」。後者由 PostgreSQL 的定義保證：
 * {@code NULLIF(a, b)} 在 {@code a = b} 時回 NULL，否則回 {@code a}。
 */
class ShipmentAnomalyQueryMapperSqlTest {

    /** SQL 在 {@code <script>} 內以 XML 解析，比對前先把換行與連續空白壓成單一空格。 */
    private static String normalized() {
        return ShipmentAnomalyQueryMapper.CASE_ANOMALY_TYPE.replaceAll("\\s+", " ");
    }

    /** 只取 {@code PAYUNI_LGS_NO_CODE} 那一條 WHEN 分支，避免斷言誤命中別條分支的述詞。 */
    private static String payuniLgsNoCodeBranch() {
        String sql = normalized();
        int end = sql.indexOf("THEN 'PAYUNI_LGS_NO_CODE'");
        assertTrue(end > 0, "找不到 PAYUNI_LGS_NO_CODE 分支——CASE 片段被改動過，先確認本測試還指著對的東西");
        int start = sql.lastIndexOf("WHEN", end);
        assertTrue(start >= 0 && start < end, "找不到該分支的 WHEN 起點");
        return sql.substring(start, end);
    }

    @Test
    void payuniLgsNoCode_treatsEmptyStringAsMissing_notOnlyNull() {
        String branch = payuniLgsNoCodeBranch();
        for (String column : new String[]{"s.partner_id", "s.odno", "s.validation_no"}) {
            assertTrue(branch.contains("NULLIF(" + column + ", '') IS NULL"),
                    "三段碼欄位 " + column + " 必須以 NULLIF(欄位, '') IS NULL 判定，"
                            + "否則 webhook 寫進來的空字串永遠不會被判成缺失。實際分支：" + branch);
        }
    }

    @Test
    void payuniLgsNoCode_hasNoBareIsNullOnThreeSegmentColumns() {
        String branch = payuniLgsNoCodeBranch();
        for (String column : new String[]{"s.partner_id", "s.odno", "s.validation_no"}) {
            assertFalse(branch.contains(column + " IS NULL"),
                    "裸 " + column + " IS NULL 會漏掉空字串（'' IS NULL 恆為 false）。實際分支：" + branch);
        }
    }
}
