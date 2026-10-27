package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * {@code order_invoice_snapshots} Mapper（58 號 §5.4.3、Flyway V12）。
 *
 * <p>三條路徑，<b>全部是 INSERT 或 SELECT，沒有 UPDATE 也不會有</b>（58 號 I-50）：
 * 建單交易內 INSERT v1（{@code BaseMapper.insert}）、invoice-service 經 gRPC 讀取、
 * B06／R03 更正時 INSERT v(n+1)（{@code ReviseOrderInvoiceSnapshotInternal}，§6.2.5）。
 *
 * <p>⚠️ I-50 的「唯一寫入者」講的是<b>服務</b>（order-service）不是<b>路徑</b>。
 * 該表 append-only，讀端一律取 {@code snapshot_version} 最大的那一列。
 */
@Mapper
public interface OrderInvoiceSnapshotRepository extends BaseMapper<OrderInvoiceSnapshotEntity> {

    /**
     * 取該訂單目前最新版的快照。
     *
     * <p>{@code SELECT *}：回傳型別是 Entity，依 §11.3 partial projection 禁令必須涵蓋全部欄位。
     * {@code ORDER BY snapshot_version DESC} 與 {@code uk_order_invoice_snapshots}
     * 的前綴一致；今天恆只有一列，寫成這樣是為了「未來真的有 v2 時讀到的是 v2」而不是
     * 依賴 PostgreSQL 不保證的回傳順序。
     */
    @Select("""
            SELECT * FROM order_invoice_snapshots
            WHERE order_no = #{orderNo}
            ORDER BY snapshot_version DESC
            LIMIT 1
            """)
    OrderInvoiceSnapshotEntity findLatestByOrderNo(@Param("orderNo") String orderNo);

    /**
     * 依更正冪等鍵取回既有的那一列（58 號 §6.2.5 定版 3）。
     *
     * <p>命中即代表這把鑰匙先前已經成功寫入過 ⇒ 呼叫端回<b>同一個</b>
     * {@code snapshot_version}、不再 INSERT。沒有這一支，invoice-service 的 gRPC 重試
     * 會每次遞增版本，而版本號是開票冪等鍵的組成之一（§11 兩軸表）。
     *
     * <p>{@code SELECT *}：回傳型別是 Entity，依 §11.3 partial projection 禁令必須涵蓋全部欄位。
     */
    @Select("""
            SELECT * FROM order_invoice_snapshots
            WHERE order_no = #{orderNo}
              AND revision_idempotency_key = #{idempotencyKey}
            """)
    OrderInvoiceSnapshotEntity findByRevisionIdempotencyKey(@Param("orderNo") String orderNo,
                                                            @Param("idempotencyKey") String idempotencyKey);
}
