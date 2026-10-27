package io.github.will413028.hawkr.product.entity.projection;

import java.time.LocalDateTime;

/**
 * 管理／稽核視角的商品輕量 projection。
 *
 * <p>與 {@link ProductBriefProjection} 的差別只有一個，但那個差別是本 projection 存在的理由：
 * 它**不套任何可見性過濾**，已軟刪的商品也回得出來。對齊
 * {@code ProductRepository#findByIdIncludingDeleted} 的語意——**取名字不該被可見性過濾**。
 *
 * <p>三個時間欄不是裝飾：呼叫端要用 {@code ProductPublicationPolicy#deriveState} 算出
 * {@code ProductPublicationState}，而那支需要 {@code firstPublishedAt}／{@code deletedAt}／
 * {@code sellerDeletedAt} 三者齊全。**不要**在 SQL 或呼叫端自己寫
 * 「deletedAt != null || sellerDeletedAt != null」——那是同一條規則的第二份定義。
 *
 * @param id               product TSID
 * @param name             商品名稱
 * @param status           {@link io.github.will413028.hawkr.common.utils.code.ProductStatus#value()}
 * @param firstPublishedAt 首次上架時間；null = 從未公開
 * @param deletedAt        商品軟刪時間戳
 * @param sellerDeletedAt  所屬賣家帳號刪除時間戳（V7；與 deletedAt 是兩個獨立維度）
 * @param storeExposable   所屬店家目前是否可曝光（{@code store_exposure}，沒有列 = false），
 *                         交給 {@code deriveState} 的 {@code storeExposable} 參數
 */
public record ProductPublicationBriefProjection(
        Long id,
        String name,
        String status,
        LocalDateTime firstPublishedAt,
        LocalDateTime deletedAt,
        LocalDateTime sellerDeletedAt,
        boolean storeExposable) {}
