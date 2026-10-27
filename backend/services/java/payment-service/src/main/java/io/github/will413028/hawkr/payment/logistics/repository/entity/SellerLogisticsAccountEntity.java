package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code seller_logistics_accounts}（V9）—— 賣家層級的物流費帳戶。
 *
 * <p>🔴 主鍵是 {@code store_id}（賣家業務鍵，非 account_id），<b>不是 TSID 也不是 bigserial</b>
 * ⇒ {@link IdType#INPUT}。
 *
 * <p>🔴 {@code receivableBalance} 與 {@code creditBalance} 是<b>兩個科目</b>，不是一個欄位的正負：
 * 兩者可以同時存在（這期欠、上期多收），合成淨額之後 ADR-008 第 1 條保證（永久可追蹤）就失效了。
 *
 * <p>🔴 {@code creditBalance} <b>永不過期、不因賣家無後續交易而清零</b>（ADR-008 第 2、3 條）
 * ⇒ <b>不得有任何排程把它歸零</b>。
 */
@Data
@TableName(value = "seller_logistics_accounts", autoResultMap = true)
public class SellerLogisticsAccountEntity {

    /** 自然主鍵：賣家 store surrogate。 */
    @TableId(value = "store_id", type = IdType.INPUT)
    private Long storeId;

    private Long receivableBalance;
    private Long creditBalance;

    /** 🔴 §5 帳齡階梯的錨，<b>不是「餘額為正」</b>。 */
    private LocalDateTime lastActivityAt;

    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
