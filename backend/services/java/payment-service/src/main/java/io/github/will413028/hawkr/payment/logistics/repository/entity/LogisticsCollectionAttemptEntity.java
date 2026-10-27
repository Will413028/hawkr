package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDate;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code logistics_collection_attempts}（V9）—— 一次 {@code agent_fee} 送出的紀錄。
 *
 * <p>🔴 {@code tradeNo} <b>由我方生成並保證唯一</b>（{@code uq_logcoll_trade_no}）：
 * PayUni 的 TradeNo 官方三處逐字標「可重複」，且 {@code agent_fee_query} 回<b>單筆結構非陣列</b>
 * ⇒ 真的送重複了，用官方 API 也對不出有幾筆。
 *
 * <p>🔴 {@code netProceeds} NULL ＝ 尚未查詢，<b>不是 0</b>（0 代表查到了而且真的一毛沒收到）。
 * {@code fundDate} 與 {@code actualFee} 是雙向等式（{@code chk_logcoll_settled_pair}）。
 *
 * <p>🔴 {@code queryLeaseUntil} 是<b>租約</b>，不是「已查過」旗標（10 檔 §29）。
 */
@Data
@TableName(value = "logistics_collection_attempts", autoResultMap = true)
public class LogisticsCollectionAttemptEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    private Long receivableId;
    private String tradeNo;

    /** 🔴 實際送出的 Amt（adapter 依費率反推），值域 1~9999999。 */
    private Long grossAmount;

    /** 🔴 NULL ＝ 尚未查詢，不是 0。 */
    private Long netProceeds;

    /** 🔴 {@code target_net − net_proceeds}，可為負；不為 0 不得標記已收訖。 */
    private Long residual;

    private String submitStatus;
    private String submitMessage;
    private LocalDateTime submittedAt;

    /** 🔴 送出日 ＋ 1 個工作日，UNCONFIRMED 判定的錨。 */
    private LocalDate expectedFundDate;

    /** 來自 {@code agent_fee_query}；{@code '0000-00-00'} 一律映成 NULL。 */
    private LocalDate fundDate;

    /** 🔴 實際手續費，不得用預估值冒充。 */
    private Long actualFee;

    /** 🔴 租約欄，不是旗標。 */
    private LocalDateTime queryLeaseUntil;

    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
