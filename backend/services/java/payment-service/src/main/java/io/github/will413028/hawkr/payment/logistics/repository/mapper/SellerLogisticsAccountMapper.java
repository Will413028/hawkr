package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code seller_logistics_accounts} 的 mapper。
 *
 * <p>🔴 兩個餘額是<b>兩個科目</b>，不得合成淨額（ADR-008 第 1 條保證的機械形式）。
 * 本 mapper 因此<b>刻意沒有</b>「設定淨額」或「重設餘額」的方法：只有兩支各自的 delta。
 */
@Mapper
public interface SellerLogisticsAccountMapper extends BaseMapper<SellerLogisticsAccountEntity> {

    /** 確保帳戶存在（冪等）；已存在時只推進 {@code last_activity_at}。 */
    @Insert("""
            INSERT INTO seller_logistics_accounts (store_id, receivable_balance, credit_balance, last_activity_at)
            VALUES (#{storeId}, 0, 0, NOW())
            ON CONFLICT (store_id) DO UPDATE
                SET last_activity_at = NOW(), updated_at = NOW()
            """)
    int ensureExists(@Param("storeId") Long storeId);

    @Select("SELECT * FROM seller_logistics_accounts WHERE store_id = #{storeId}")
    SellerLogisticsAccountEntity findByStoreId(@Param("storeId") Long storeId);

    /** 賣家欠平台的餘額 ± delta。 */
    @Update("""
            UPDATE seller_logistics_accounts
            SET receivable_balance = receivable_balance + #{delta},
                last_activity_at = NOW(),
                updated_at = NOW()
            WHERE store_id = #{storeId}
            """)
    int addReceivableBalance(@Param("storeId") Long storeId, @Param("delta") Long delta);

    /**
     * 平台欠賣家的貸記 ± delta。
     *
     * <p>🔴 {@code credit_balance} <b>永不過期、不因賣家無後續交易而清零</b>（ADR-008 第 2、3 條）
     * ⇒ <b>不得有任何排程呼叫本方法帶負值把它歸零</b>。負 delta 只允許來自
     * {@code LOGISTICS_CREDIT_APPLY}（真的抵扣掉了），且 {@code chk_sla_credit} 保證不會變成負數
     * ——抵扣超額會被 DB 直接擋下，而不是靜默變負。
     */
    @Update("""
            UPDATE seller_logistics_accounts
            SET credit_balance = credit_balance + #{delta},
                last_activity_at = NOW(),
                updated_at = NOW()
            WHERE store_id = #{storeId}
            """)
    int addCreditBalance(@Param("storeId") Long storeId, @Param("delta") Long delta);
}
