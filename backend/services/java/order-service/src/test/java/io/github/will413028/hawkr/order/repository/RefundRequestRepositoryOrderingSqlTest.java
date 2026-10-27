package io.github.will413028.hawkr.order.repository;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@link RefundRequestRepository} 的排序**語意**錨（issue #2547）。
 *
 * <p>「排序鍵有沒有構成全序」由 {@link RepositoryOrderingSqlTest} 全 package 統一守；本檔守的是
 * 這個 mapper 特有的一條不變式：<b>{@link RefundRequestRepository#findByOrderNo} 的第一個排序鍵
 * 必須是 {@code created_at}</b>。
 *
 * <p>退款歷程是時序性資料——{@code OrderProtoConverter} 依 List 順序逐筆 append 進
 * {@code refund_history}，Gateway 原序透傳，前端直接畫成時間軸。而主鍵 {@code request_no}
 * 不是本服務產生的，字典序不保證等於時序：拿它當首鍵，時間軸會排出一個**決定性但與時間無關**的
 * 順序——這種錯比沒有排序更難發現，因為它每次重整都長得一樣。
 */
class RefundRequestRepositoryOrderingSqlTest {

    @Test
    @DisplayName("findByOrderNo 以 created_at 為首鍵（時間軸的順序要真的是時間）")
    void refundHistoryIsOrderedByTimeNotByPrimaryKey() {
        String orderBy = MapperSql.orderByOf(
                MapperSql.select(RefundRequestRepository.class, "findByOrderNo", String.class));

        assertThat(orderBy)
                .as("findByOrderNo 沒有 ORDER BY ⇒ 這個錨失效了，不是通過")
                .isNotNull();
        assertThat(orderBy)
                .as("退款歷程的首鍵不是 created_at ⇒ request_no 非本服務產生，"
                        + "字典序不等於時序，時間軸會排出與時間無關的順序")
                .containsPattern("^(\\w+\\.)?created_at(?![_a-zA-Z])");
    }
}
