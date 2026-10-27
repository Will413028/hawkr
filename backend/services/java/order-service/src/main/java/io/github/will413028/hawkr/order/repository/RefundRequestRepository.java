package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.entity.RefundRequestEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.List;

/**
 * 退款申請表 Mapper
 */
public interface RefundRequestRepository extends BaseMapper<RefundRequestEntity> {

    @Select("SELECT * FROM refund_requests WHERE request_no = #{requestNo}")
    RefundRequestEntity findByRequestNo(@Param("requestNo") String requestNo);

    @Select("SELECT * FROM refund_requests WHERE request_no = #{requestNo} FOR UPDATE")
    RefundRequestEntity findByRequestNoForUpdate(@Param("requestNo") String requestNo);

    /**
     * 退款申請歷程（訂單詳情的時間軸，四支 RPC 共用）。
     *
     * <p>排序鍵是 created_at 而非主鍵：request_no 由本服務以外的路徑產生，字典序不保證等於時序。
     * 收尾補主鍵是因為 created_at 取 NOW()＝交易開始時刻，同一交易內建立的多筆退款申請時間戳完全
     * 相同，只靠 created_at 仍然沒有全序。
     */
    @Select("SELECT * FROM refund_requests WHERE order_no = #{orderNo} ORDER BY created_at ASC, request_no ASC")
    List<RefundRequestEntity> findByOrderNo(@Param("orderNo") String orderNo);

    @Update("""
        UPDATE refund_requests
        SET status = #{status}, updated_at = NOW()
        WHERE request_no = #{requestNo}
    """)
    int updateStatus(@Param("requestNo") String requestNo, @Param("status") String status);
}
