package io.github.will413028.hawkr.order.repository;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@link OrderItemRepository} 的排序**語意**錨（issue #2547）。
 *
 * <p>「排序鍵有沒有構成全序」由 {@link RepositoryOrderingSqlTest} 全 package 統一守；本檔守的是
 * 這個 mapper 特有的一條不變式：<b>{@link OrderItemRepository#findByOrderNo} 與
 * {@link OrderItemRepository#findFirstByOrderNo} 的排序鍵必須逐字一致</b>。
 *
 * <p>兩者都在回答「第一件是哪一件」——前者給訂單列表與售後（消費端取 {@code items[0]} 當
 * gateway 的 {@code firstProductName}），後者給 Payment 取 {@code product_name}。排序鍵一旦分岔，
 * 同一筆訂單在兩個畫面上的「第一件」就是不同商品，而且不報錯、不轉紅，只是兩邊對不起來。
 */
class OrderItemRepositoryOrderingSqlTest {

    @Test
    @DisplayName("findByOrderNo 與 findFirstByOrderNo 的排序鍵逐字一致")
    void firstItemMeansTheSameThingInBothQueries() {
        String all = MapperSql.orderByOf(
                MapperSql.select(OrderItemRepository.class, "findByOrderNo", String.class));
        String first = MapperSql.orderByOf(
                MapperSql.select(OrderItemRepository.class, "findFirstByOrderNo", String.class));

        assertThat(all)
                .as("findByOrderNo 沒有 ORDER BY ⇒ 它的 items[0] 與 findFirstByOrderNo 隨時可能是不同商品")
                .isNotNull();
        assertThat(first)
                .as("findFirstByOrderNo 沒有 ORDER BY ⇒ 這個錨失效了，不是通過")
                .isNotNull();
        assertThat(all)
                .as("兩支查詢都在回答「第一件是哪一件」，排序鍵分岔＝同一筆訂單在兩個畫面上的第一件不同")
                .isEqualTo(first);
    }
}
