package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.SellerCancelRequestEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Update;

@Mapper
public interface SellerCancelRequestMapper extends BaseMapper<SellerCancelRequestEntity> {

    /**
     * §2.3 INV-3 / INV-4 對齊：退款成功後完成 cancel request（REFUND_PROCESSING 10 → COMPLETED 20）。
     * 由 RefundResultConsumer.closeParentAndPublish 觸發；webhook 重投或多 path race 時冪等 skip。
     * WHERE status IN ('10') INV-4 收緊：COMPLETED 唯一合法前置為 REFUND_PROCESSING；
     * 未付款 0 元場景（PENDING → COMPLETED）走 markCompletedFromPending 不共用本 mapper。
     *
     * @return 1 = 完成成功；0 = 已完成或前置不合法，consumer 應 ACK skip
     */
    @Update("""
            UPDATE seller_cancel_requests
               SET status = '20'
             WHERE id = #{id}
               AND status IN ('10')
            """)
    int completeIfNotCompleted(@Param("id") Long id);

    /**
     * §2.3 INV-3：已付款場景 PENDING(0) → REFUND_PROCESSING(10)。
     * 由 SellerCancelService.cancelShipment* 在建退款任務後呼叫。
     */
    @Update("""
            UPDATE seller_cancel_requests
               SET status = '10'
             WHERE id = #{id}
               AND status IN ('0')
            """)
    int markRefundProcessing(@Param("id") Long id);

    /**
     * §2.3 INV-3：未付款 0 元場景 PENDING(0) → COMPLETED(20)。
     * 由 SellerCancelService.cancelShipment* 在 isUnpaid 分支呼叫；無退款任務需執行。
     */
    @Update("""
            UPDATE seller_cancel_requests
               SET status = '20'
             WHERE id = #{id}
               AND status IN ('0')
            """)
    int markCompletedFromPending(@Param("id") Long id);

    /**
     * §2.3 INV-3：失敗場景 PENDING(0) → FAILED(30)。
     * 由 SellerCancelService.cancelShipment* 在 cancelOrderInternal RPC 失敗或 ATM createRefund 失敗時呼叫。
     */
    @Update("""
            UPDATE seller_cancel_requests
               SET status = '30'
             WHERE id = #{id}
               AND status IN ('0')
            """)
    int markFailedFromPending(@Param("id") Long id);
}
