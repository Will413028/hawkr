package io.github.will413028.hawkr.cart.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.CartItemStatus;
import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import org.apache.ibatis.annotations.*;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 購物車明細 Mapper
 *
 * 包含原子 Upsert、條件鎖定等自訂 SQL
 */
public interface CartItemRepository extends BaseMapper<CartItemEntity> {

    // ==================== AddItem 原子 Upsert ====================

    default int upsertMemberItem(CartItemEntity entity) {
        return upsertMemberItemInternal(entity, CartItemStatus.NORMAL.value());
    }

    @Insert("""
        INSERT INTO cart_items (id, buyer_id, store_id, sku_id, quantity,
                                added_unit_price, added_list_price,
                                max_limit_snapshot, sku_name_snapshot,
                                product_id, spec_name_snapshot, image_object_key, status)
        VALUES (#{entity.id}, #{entity.buyerId}, #{entity.storeId}, #{entity.skuId},
                #{entity.quantity}, #{entity.addedUnitPrice}, #{entity.addedListPrice},
                #{entity.maxLimitSnapshot}, #{entity.skuNameSnapshot},
                #{entity.productId}, #{entity.specNameSnapshot}, #{entity.imageObjectKey}, #{statusNormal})
        ON CONFLICT (buyer_id, sku_id) WHERE buyer_id IS NOT NULL
        DO UPDATE SET
            quantity = LEAST(cart_items.quantity + EXCLUDED.quantity, EXCLUDED.max_limit_snapshot),
            max_limit_snapshot = EXCLUDED.max_limit_snapshot,
            sku_name_snapshot = EXCLUDED.sku_name_snapshot,
            product_id = EXCLUDED.product_id,
            spec_name_snapshot = EXCLUDED.spec_name_snapshot,
            image_object_key = EXCLUDED.image_object_key
            -- added_unit_price / added_list_price 在 Upsert 衝突時不更新，符合「快照不刷新」原則（04 設計書 §A.1）
        WHERE cart_items.status = #{statusNormal}
        """)
    int upsertMemberItemInternal(@Param("entity") CartItemEntity entity,
                                  @Param("statusNormal") String statusNormal);

    default int upsertGuestItem(CartItemEntity entity) {
        return upsertGuestItemInternal(entity, CartItemStatus.NORMAL.value());
    }

    @Insert("""
        INSERT INTO cart_items (id, guest_id, store_id, sku_id, quantity,
                                added_unit_price, added_list_price,
                                max_limit_snapshot, sku_name_snapshot,
                                product_id, spec_name_snapshot, image_object_key, status)
        VALUES (#{entity.id}, #{entity.guestId}, #{entity.storeId}, #{entity.skuId},
                #{entity.quantity}, #{entity.addedUnitPrice}, #{entity.addedListPrice},
                #{entity.maxLimitSnapshot}, #{entity.skuNameSnapshot},
                #{entity.productId}, #{entity.specNameSnapshot}, #{entity.imageObjectKey}, #{statusNormal})
        ON CONFLICT (guest_id, sku_id) WHERE guest_id IS NOT NULL
        DO UPDATE SET
            quantity = LEAST(cart_items.quantity + EXCLUDED.quantity, EXCLUDED.max_limit_snapshot),
            max_limit_snapshot = EXCLUDED.max_limit_snapshot,
            sku_name_snapshot = EXCLUDED.sku_name_snapshot,
            product_id = EXCLUDED.product_id,
            spec_name_snapshot = EXCLUDED.spec_name_snapshot,
            image_object_key = EXCLUDED.image_object_key
            -- added_unit_price / added_list_price 在 Upsert 衝突時不更新（同上）
        WHERE cart_items.status = #{statusNormal}
        """)
    int upsertGuestItemInternal(@Param("entity") CartItemEntity entity,
                                 @Param("statusNormal") String statusNormal);

    // ==================== 查詢 ====================

    /** 查詢會員購物車所有項目（依 store_id 排序） */
    @Select("SELECT * FROM cart_items WHERE buyer_id = #{buyerId} ORDER BY store_id, created_at")
    List<CartItemEntity> findByBuyerId(@Param("buyerId") Long buyerId);

    /** 查詢訪客購物車所有項目（依 store_id 排序） */
    @Select("SELECT * FROM cart_items WHERE guest_id = #{guestId} ORDER BY store_id, created_at")
    List<CartItemEntity> findByGuestId(@Param("guestId") String guestId);

    /** 查詢會員 LOCKED 項目（GetCart 合併用，不進 Redis） */
    default List<CartItemEntity> findLockedByBuyerId(Long buyerId) {
        return findByBuyerIdAndStatusLocked(buyerId, CartItemStatus.LOCKED.value());
    }

    @Select("""
        SELECT * FROM cart_items
        WHERE buyer_id = #{buyerId} AND status = #{status} AND lock_expire_time > NOW()
        ORDER BY store_id, created_at
        """)
    List<CartItemEntity> findByBuyerIdAndStatusLocked(@Param("buyerId") Long buyerId,
                                                       @Param("status") String status);

    /** 查詢訪客 LOCKED 項目 */
    default List<CartItemEntity> findLockedByGuestId(String guestId) {
        return findByGuestIdAndStatusLocked(guestId, CartItemStatus.LOCKED.value());
    }

    @Select("""
        SELECT * FROM cart_items
        WHERE guest_id = #{guestId} AND status = #{status} AND lock_expire_time > NOW()
        ORDER BY store_id, created_at
        """)
    List<CartItemEntity> findByGuestIdAndStatusLocked(@Param("guestId") String guestId,
                                                       @Param("status") String status);

    /** 查詢會員 NORMAL 項目（Redis 回填用） */
    default List<CartItemEntity> findNormalByBuyerId(Long buyerId) {
        return findByBuyerIdAndStatusNormal(buyerId, CartItemStatus.NORMAL.value());
    }

    @Select("SELECT * FROM cart_items WHERE buyer_id = #{buyerId} AND status = #{status} ORDER BY store_id, created_at")
    List<CartItemEntity> findByBuyerIdAndStatusNormal(@Param("buyerId") Long buyerId,
                                                       @Param("status") String status);

    /** 查詢訪客 NORMAL 項目 */
    default List<CartItemEntity> findNormalByGuestId(String guestId) {
        return findByGuestIdAndStatusNormal(guestId, CartItemStatus.NORMAL.value());
    }

    @Select("SELECT * FROM cart_items WHERE guest_id = #{guestId} AND status = #{status} ORDER BY store_id, created_at")
    List<CartItemEntity> findByGuestIdAndStatusNormal(@Param("guestId") String guestId,
                                                       @Param("status") String status);

    /** 計算 owner 的品項數（含 NORMAL + LOCKED） */
    @Select("SELECT COUNT(*) FROM cart_items WHERE buyer_id = #{buyerId}")
    int countByBuyerId(@Param("buyerId") Long buyerId);

    @Select("SELECT COUNT(*) FROM cart_items WHERE guest_id = #{guestId}")
    int countByGuestId(@Param("guestId") String guestId);

    /** 查詢會員特定 SKU */
    @Select("SELECT * FROM cart_items WHERE buyer_id = #{buyerId} AND sku_id = #{skuId}")
    CartItemEntity findByBuyerIdAndSkuId(@Param("buyerId") Long buyerId, @Param("skuId") Long skuId);

    /** 查詢訪客特定 SKU */
    @Select("SELECT * FROM cart_items WHERE guest_id = #{guestId} AND sku_id = #{skuId}")
    CartItemEntity findByGuestIdAndSkuId(@Param("guestId") String guestId, @Param("skuId") Long skuId);

    // ==================== SetItemQuantity ====================

    /** 更新會員項目數量（需檢查 status=NORMAL） */
    default int updateQuantityByBuyerSku(Long buyerId, Long skuId, int quantity) {
        return updateQuantityByBuyerSkuInternal(buyerId, skuId, quantity, CartItemStatus.NORMAL.value());
    }

    @Update("""
        UPDATE cart_items SET quantity = #{quantity}
        WHERE buyer_id = #{buyerId} AND sku_id = #{skuId} AND status = #{statusNormal}
        """)
    int updateQuantityByBuyerSkuInternal(@Param("buyerId") Long buyerId,
                                          @Param("skuId") Long skuId,
                                          @Param("quantity") int quantity,
                                          @Param("statusNormal") String statusNormal);

    /** 更新訪客項目數量 */
    default int updateQuantityByGuestSku(String guestId, Long skuId, int quantity) {
        return updateQuantityByGuestSkuInternal(guestId, skuId, quantity, CartItemStatus.NORMAL.value());
    }

    @Update("""
        UPDATE cart_items SET quantity = #{quantity}
        WHERE guest_id = #{guestId} AND sku_id = #{skuId} AND status = #{statusNormal}
        """)
    int updateQuantityByGuestSkuInternal(@Param("guestId") String guestId,
                                          @Param("skuId") Long skuId,
                                          @Param("quantity") int quantity,
                                          @Param("statusNormal") String statusNormal);

    /** 刪除會員特定 SKU（qty=0 時） */
    default int deleteByBuyerSku(Long buyerId, Long skuId) {
        return deleteByBuyerSkuInternal(buyerId, skuId, CartItemStatus.NORMAL.value());
    }

    @Delete("DELETE FROM cart_items WHERE buyer_id = #{buyerId} AND sku_id = #{skuId} AND status = #{statusNormal}")
    int deleteByBuyerSkuInternal(@Param("buyerId") Long buyerId,
                                  @Param("skuId") Long skuId,
                                  @Param("statusNormal") String statusNormal);

    /** 刪除訪客特定 SKU */
    default int deleteByGuestSku(String guestId, Long skuId) {
        return deleteByGuestSkuInternal(guestId, skuId, CartItemStatus.NORMAL.value());
    }

    @Delete("DELETE FROM cart_items WHERE guest_id = #{guestId} AND sku_id = #{skuId} AND status = #{statusNormal}")
    int deleteByGuestSkuInternal(@Param("guestId") String guestId,
                                  @Param("skuId") Long skuId,
                                  @Param("statusNormal") String statusNormal);

    // ==================== CheckoutLock / Unlock / Remove ====================

    default int lockItemsByBuyerAndSkus(Long buyerId, String checkoutId,
                                         LocalDateTime lockExpireTime, List<Long> skuIds) {
        return lockItemsByBuyerAndSkusInternal(buyerId, checkoutId, lockExpireTime, skuIds,
                CartItemStatus.LOCKED.value(), CartItemStatus.NORMAL.value());
    }

    @Update("""
        <script>
        UPDATE cart_items
        SET status = #{statusLocked}, checkout_id = #{checkoutId}, lock_expire_time = #{lockExpireTime}
        WHERE buyer_id = #{buyerId}
        AND sku_id IN
        <foreach item="skuId" collection="skuIds" open="(" separator="," close=")">
            #{skuId}
        </foreach>
        AND (status = #{statusNormal} OR (status = #{statusLocked} AND checkout_id = #{checkoutId}))
        </script>
        """)
    int lockItemsByBuyerAndSkusInternal(@Param("buyerId") Long buyerId,
                                         @Param("checkoutId") String checkoutId,
                                         @Param("lockExpireTime") LocalDateTime lockExpireTime,
                                         @Param("skuIds") List<Long> skuIds,
                                         @Param("statusLocked") String statusLocked,
                                         @Param("statusNormal") String statusNormal);

    /**
     * 解除鎖定：依 checkout_id 解鎖指定 SKU（補償用，冪等）
     */
    default int unlockItemsByCheckoutId(Long buyerId, String checkoutId, List<Long> skuIds) {
        return unlockItemsByCheckoutIdInternal(buyerId, checkoutId, skuIds,
                CartItemStatus.NORMAL.value(), CartItemStatus.LOCKED.value());
    }

    @Update("""
        <script>
        UPDATE cart_items
        SET status = #{statusNormal}, checkout_id = NULL, lock_expire_time = NULL
        WHERE buyer_id = #{buyerId} AND checkout_id = #{checkoutId} AND status = #{statusLocked}
        <if test="skuIds != null and skuIds.size() > 0">
        AND sku_id IN
        <foreach item="skuId" collection="skuIds" open="(" separator="," close=")">
            #{skuId}
        </foreach>
        </if>
        </script>
        """)
    int unlockItemsByCheckoutIdInternal(@Param("buyerId") Long buyerId,
                                         @Param("checkoutId") String checkoutId,
                                         @Param("skuIds") List<Long> skuIds,
                                         @Param("statusNormal") String statusNormal,
                                         @Param("statusLocked") String statusLocked);

    /**
     * 結帳成功後移除：依 checkout_id 刪除（冪等）
     */
    @Delete("""
        <script>
        DELETE FROM cart_items
        WHERE buyer_id = #{buyerId} AND checkout_id = #{checkoutId}
        <if test="skuIds != null and skuIds.size() > 0">
        AND sku_id IN
        <foreach item="skuId" collection="skuIds" open="(" separator="," close=")">
            #{skuId}
        </foreach>
        </if>
        </script>
        """)
    int removeByCheckoutId(@Param("buyerId") Long buyerId,
                           @Param("checkoutId") String checkoutId,
                           @Param("skuIds") List<Long> skuIds);

    // ==================== MergeCart ====================

    /** 查詢訪客所有 NORMAL 項目（合併用） */
    default List<CartItemEntity> findNormalByGuestIdForMerge(String guestId) {
        return findNormalByGuestIdForMergeInternal(guestId, CartItemStatus.NORMAL.value());
    }

    @Select("SELECT * FROM cart_items WHERE guest_id = #{guestId} AND status = #{statusNormal}")
    List<CartItemEntity> findNormalByGuestIdForMergeInternal(@Param("guestId") String guestId,
                                                              @Param("statusNormal") String statusNormal);

    /** 轉移訪客項目至會員（MergeCart 用，明確 SET guest_id = NULL 避免 MyBatis-Plus 忽略 null） */
    @Update("UPDATE cart_items SET buyer_id = #{buyerId}, guest_id = NULL WHERE id = #{id}")
    int transferOwnerToMember(@Param("id") Long id, @Param("buyerId") Long buyerId);

    /** 自買過濾：刪除訪客車中由 storeIds（已解析的現任 store surrogate）自己上架的 NORMAL 商品（MergeCart 用，§5.3） */
    default int deleteSelfOwnedGuestItems(String guestId, List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return 0;
        }
        return deleteSelfOwnedGuestItemsInternal(guestId, storeIds, CartItemStatus.NORMAL.value());
    }

    @Delete("""
        <script>
        DELETE FROM cart_items WHERE guest_id = #{guestId} AND status = #{statusNormal}
        AND store_id IN
        <foreach item="storeId" collection="storeIds" open="(" separator="," close=")">
            #{storeId}
        </foreach>
        </script>
        """)
    int deleteSelfOwnedGuestItemsInternal(@Param("guestId") String guestId,
                                          @Param("storeIds") List<Long> storeIds,
                                          @Param("statusNormal") String statusNormal);

    /** 刪除訪客所有項目（合併後清理） */
    @Delete("DELETE FROM cart_items WHERE guest_id = #{guestId}")
    int deleteAllByGuestId(@Param("guestId") String guestId);

    // ==================== 背景 Job ====================

    /** 解鎖過期的 LOCKED 項目（設計書 §7.1） */
    default int unlockExpiredItems() {
        return unlockExpiredItemsInternal(CartItemStatus.NORMAL.value(), CartItemStatus.LOCKED.value());
    }

    @Update("UPDATE cart_items SET status = #{statusNormal}, checkout_id = NULL, lock_expire_time = NULL WHERE status = #{statusLocked} AND lock_expire_time <= NOW()")
    int unlockExpiredItemsInternal(@Param("statusNormal") String statusNormal,
                                    @Param("statusLocked") String statusLocked);

    /** 清理超過指定天數的訪客購物車資料（設計書 §7.2） */
    @Delete("DELETE FROM cart_items WHERE guest_id IS NOT NULL AND updated_at < #{cutoffTime}")
    int deleteExpiredGuestItems(@Param("cutoffTime") LocalDateTime cutoffTime);
}
