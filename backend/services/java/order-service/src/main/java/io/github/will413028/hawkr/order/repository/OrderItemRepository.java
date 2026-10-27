package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.List;

/**
 * 訂單明細表 Mapper
 */
public interface OrderItemRepository extends BaseMapper<OrderItemEntity> {

    /**
     * 取得訂單的全部明細。
     * <p>
     * 排序鍵與 {@link #findFirstByOrderNo} 一致，兩處取到的「第一件」才會是同一件：
     * 沒有 ORDER BY 時 PostgreSQL 不保證回傳順序，第一件退款觸發 UPDATE 之後 MVCC 換 tuple 位置，
     * 同一筆訂單重新整理就會顯示不同品名。
     */
    @Select("SELECT * FROM order_items WHERE order_no = #{orderNo} ORDER BY id ASC")
    List<OrderItemEntity> findByOrderNo(@Param("orderNo") String orderNo);

    @Select("SELECT * FROM order_items WHERE id = #{id}")
    OrderItemEntity findById(@Param("id") long id);

    @Select("SELECT * FROM order_items WHERE id = #{id} FOR UPDATE")
    OrderItemEntity findByIdForUpdate(@Param("id") long id);

    @Update("UPDATE order_items SET refund_status = #{refundStatus} WHERE id = #{id}")
    int updateRefundStatus(@Param("id") long id, @Param("refundStatus") String refundStatus);

    @Select("SELECT COUNT(*) FROM order_items WHERE order_no = #{orderNo} AND refund_status IN ('20', '21')")
    int countRefundedByOrderNo(@Param("orderNo") String orderNo);

    @Select("SELECT COUNT(*) FROM order_items WHERE order_no = #{orderNo}")
    int countByOrderNo(@Param("orderNo") String orderNo);

    /**
     * 取得訂單的第一筆明細（供 Payment Service 取 product_name）
     */
    @Select("SELECT * FROM order_items WHERE order_no = #{orderNo} ORDER BY id ASC LIMIT 1")
    OrderItemEntity findFirstByOrderNo(@Param("orderNo") String orderNo);
}
