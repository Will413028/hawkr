package io.github.will413028.hawkr.order.repository;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.CALLS_REAL_METHODS;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.withSettings;

/**
 * 鎖住「消費紀錄納入哪些訂單狀態」這個產品決策（2026-08-05 使用者裁決：
 * 退款訂單要納入並顯示退款金額）。
 * <p>
 * 三支查詢（明細 / 筆數 / 賣家彙總）必須用<b>同一組</b>狀態：明細與筆數不一致會讓分頁
 * total 對不上 records，明細與彙總不一致會讓畫面上「逐列相加 ≠ 總額」。
 * <p>
 * 用 CALLS_REAL_METHODS 讓 default 方法真的跑，捕捉它實際傳下去的狀態集合——
 * 純 mock 只會驗到「我呼叫了我自己 stub 的東西」，對這個不變式沒有鑑別力。
 */
class SpendingRecordStatusScopeTest {

    private static final List<String> EXPECTED =
            List.of(OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value());

    private static OrderRepository realDefaults() {
        return mock(OrderRepository.class, withSettings().defaultAnswer(CALLS_REAL_METHODS));
    }

    @Test
    @DisplayName("消費紀錄明細納入 COMPLETED(50) 與 PARTIALLY_REFUNDED(60)")
    void findBuyerOrdersForSeller_usesCompletedAndPartiallyRefunded() {
        OrderRepository repo = realDefaults();

        repo.findBuyerOrdersForSeller(1001L, 2001L, 20, 0);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(repo).findBuyerOrdersForSellerByStatuses(
                anyLong(), anyLong(), captor.capture(), anyInt(), anyInt());
        assertEquals(EXPECTED, captor.getValue());
    }

    @Test
    @DisplayName("筆數查詢與明細用同一組狀態（否則分頁 total 與 records 對不上）")
    void countBuyerOrdersForSeller_sameStatusesAsList() {
        OrderRepository repo = realDefaults();

        repo.countBuyerOrdersForSeller(1001L, 2001L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(repo).countBuyerOrdersForSellerByStatuses(anyLong(), anyLong(), captor.capture());
        assertEquals(EXPECTED, captor.getValue());
    }

    @Test
    @DisplayName("會員卡彙總與明細用同一組狀態（否則逐列相加 ≠ 總額）")
    void findBuyerMerchantSummary_sameStatusesAsList() {
        OrderRepository repo = realDefaults();

        repo.findBuyerMerchantSummary(1001L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(repo).findBuyerMerchantSummaryByStatuses(anyLong(), captor.capture());
        assertEquals(EXPECTED, captor.getValue());
    }

    /**
     * FULLY_REFUNDED(70) 刻意排除：AfterSalesRefundEventHandler.applyFullRefund 只做狀態
     * CAS、<b>不累加</b> refund_total_amount，納入只會顯示「金額 X／退回 0」的錯誤資訊
     * （dev 實測 4 筆 status=70 的 refund_total_amount 全為 0）。crm-service 的
     * RecordRefund 與本 repository 的 findOrderAggregates 都是同一個判斷。
     */
    @Test
    @DisplayName("FULLY_REFUNDED(70) / CANCELLED(80) 不納入消費紀錄")
    void spendingRecordStatuses_excludeFullyRefundedAndCancelled() {
        OrderRepository repo = realDefaults();
        repo.findBuyerOrdersForSeller(1001L, 2001L, 20, 0);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(repo).findBuyerOrdersForSellerByStatuses(
                anyLong(), anyLong(), captor.capture(), anyInt(), anyInt());
        List<String> statuses = captor.getValue();

        assertFalse(statuses.contains(OrderStatus.FULLY_REFUNDED.value()),
                "70 的 refund_total_amount 未被維護，納入會顯示錯誤的「退回 0」");
        assertFalse(statuses.contains(OrderStatus.CANCELLED.value()),
                "80 從未成交，不是消費紀錄");
        assertTrue(statuses.contains(OrderStatus.PARTIALLY_REFUNDED.value()),
                "60 是「已完成訂單事後退部分款」，漏掉它就是本次要修的 bug");
    }

    /** 沒有任何一支查詢應該再有寫死單一狀態的 '50'（那正是漏掉 60 的成因）。 */
    @Test
    @DisplayName("三支查詢都走 statuses 集合，不再寫死單一狀態")
    void allThreeQueriesTakeStatusList() {
        OrderRepository repo = realDefaults();

        repo.findBuyerOrdersForSeller(1L, 2L, 1, 0);
        repo.countBuyerOrdersForSeller(1L, 2L);
        repo.findBuyerMerchantSummary(1L);

        verify(repo).findBuyerOrdersForSellerByStatuses(anyLong(), anyLong(), anyList(), anyInt(), anyInt());
        verify(repo).countBuyerOrdersForSellerByStatuses(anyLong(), anyLong(), anyList());
        verify(repo).findBuyerMerchantSummaryByStatuses(anyLong(), anyList());
    }
}
