package io.github.will413028.hawkr.order.repository;

import io.github.will413028.hawkr.common.utils.code.ReceiptStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 收貨確認軸（E 軸）的契約守護。
 *
 * <p>🔴 <b>這一軸最容易出的錯是「有入口沒有出口」</b>：送達事件把 {@code receipt_status}
 * 寫成 {@code PENDING}，然後沒有任何一句 SQL 把它推走 ⇒ 訂單軸走到 COMPLETED 了、
 * 收貨確認軸一格都沒動。而撥款資格的其中一條正是讀這一軸 ⇒ 撥款永遠不成立，
 * <b>且沒有任何錯誤訊息</b>（不是 500、不是紅燈，就只是那筆錢不會出去）。
 *
 * <p>本測試因此同時守兩件事：映射本身，以及「那句 SQL 真的有寫這一欄」。
 * 只驗映射不驗 SQL，是本 repo 已經踩過的形狀——契約測試只斷言「有呼叫」，
 * 而實際寫進去的是另一條軸的軌跡。
 */
@DisplayName("收貨確認軸（E 軸）契約")
class ReceiptAxisContractTest {

    @Nested
    @DisplayName("確認方式 → E 軸狀態的映射")
    class Mapping {

        @Test
        @DisplayName("買家主動確認 → BUYER_CONFIRMED")
        void manualIsBuyerConfirmed() {
            assertThat(OrderRepository.receiptStatusFor(OrderConstants.CONFIRM_TYPE_MANUAL))
                    .isEqualTo(ReceiptStatus.BUYER_CONFIRMED.value());
        }

        @Test
        @DisplayName("逾期自動完成 → AUTO_CONFIRMED（🔴 與買家確認刻意分開）")
        void autoIsAutoConfirmed() {
            assertThat(OrderRepository.receiptStatusFor(OrderConstants.CONFIRM_TYPE_AUTO))
                    .isEqualTo(ReceiptStatus.AUTO_CONFIRMED.value());
        }

        /**
         * 兩者分開的理由要有測試指著：撥款資格的收貨確認條件只認買家或平台的<b>明示</b>確認。
         * 合成一個值，會讓「買家從沒表示過意見」的訂單通過那一條。
         */
        @Test
        @DisplayName("兩種確認方式必須映射到不同的值")
        void manualAndAutoAreDistinct() {
            assertThat(OrderRepository.receiptStatusFor(OrderConstants.CONFIRM_TYPE_MANUAL))
                    .isNotEqualTo(OrderRepository.receiptStatusFor(OrderConstants.CONFIRM_TYPE_AUTO));
        }

        @Test
        @DisplayName("未知確認方式 fail-closed（不猜一個「像的」值）")
        void unknownConfirmTypeFailsClosed() {
            assertThatThrownBy(() -> OrderRepository.receiptStatusFor("SOMETHING_NEW"))
                    .isInstanceOf(IllegalArgumentException.class)
                    .hasMessageContaining("ReceiptStatus");
        }
    }

    @Nested
    @DisplayName("SQL 形狀：入口與出口都必須真的寫這一欄")
    class SqlShape {

        private String sqlOf(String methodName) {
            for (Method m : OrderRepository.class.getDeclaredMethods()) {
                if (!m.getName().equals(methodName)) {
                    continue;
                }
                Update u = m.getAnnotation(Update.class);
                if (u != null && u.value().length > 0) {
                    return u.value()[0];
                }
            }
            // 🔴 找不到目標必須 FAIL 不是 SKIP：方法被改名或搬走的那天，
            //    SKIP 會讓本測試靜默轉綠，而它守的東西已經不見了。
            throw new AssertionError("找不到帶 @Update 的方法 " + methodName
                    + "——它被改名或搬走了？本測試對「目標不存在」一律 FAIL，"
                    + "因為 SKIP 會讓守護在目標消失的那一刻靜默失效");
        }

        @Test
        @DisplayName("入口：送達事件必須寫 receipt_status（否則 DB CHECK 直接 23514）")
        void deliveredWritesReceiptStatus() {
            String sql = sqlOf("updateOrderDelivered");
            assertThat(sql)
                    .as("delivered_at 與 receipt_status 必須同句寫入："
                            + "chk_orders_receipt_axis_present 要求 delivered_at 非 NULL ⇒ receipt_status 非 NULL，"
                            + "少了這一欄，每一次送達事件都會 23514 失敗，而失敗被 MQ 重試包住 ⇒ 對呼叫端完全靜默")
                    .contains("delivered_at")
                    .contains("receipt_status")
                    .contains("receipt_status_entered_at");
        }

        @Test
        @DisplayName("出口：完成訂單必須同句推進 receipt_status")
        void completionAdvancesReceiptStatus() {
            String sql = sqlOf("completeDeliveredOrderByStatus");
            assertThat(sql)
                    .as("訂單軸推進到 COMPLETED 時，收貨確認軸必須同句前進。"
                            + "少了它，E 軸永遠停在 PENDING，而撥款資格讀的正是這一軸 ⇒ 撥款永不成立且無告警")
                    .contains("receipt_status")
                    .contains("receipt_status_entered_at");
        }

        /**
         * 🔴 錨在「同一句」而不是「有寫到」：兩軸分兩句寫，中間任何一次失敗都會讓它們分歧，
         * 而分歧之後沒有任何東西會發現（沒有跨軸的 CHECK）。
         */
        @Test
        @DisplayName("兩軸必須在同一句 UPDATE 內（不得拆兩句）")
        void bothAxesInSameStatement() {
            String sql = sqlOf("completeDeliveredOrderByStatus");
            assertThat(sql.split("(?i)UPDATE\\s+orders").length - 1)
                    .as("completeDeliveredOrderByStatus 應該只有一句 UPDATE orders")
                    .isEqualTo(1);
            assertThat(sql).contains("status = #{completed}");
            assertThat(sql).contains("receipt_status = #{receiptStatus}");
        }

        @Test
        @DisplayName("E 軸的值不得是 SQL 內的字面值（值域唯一來源是 CodeMeta）")
        void noLiteralReceiptStatusInSql() {
            for (String method : new String[]{"updateOrderDelivered", "completeDeliveredOrderByStatus"}) {
                String sql = sqlOf(method);
                for (ReceiptStatus rs : ReceiptStatus.values()) {
                    assertThat(sql)
                            .as(method + " 的 SQL 內出現 E 軸狀態的字面值 '" + rs.value() + "'——"
                                    + "值域的唯一來源是 ReceiptStatus，字面值會在下次擴充值域時漂移")
                            .doesNotContain("'" + rs.value() + "'");
                }
            }
        }
    }
}
