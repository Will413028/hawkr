package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.SellerLogisticsAccountMapper;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * 賣家物流費帳戶的 repository（thin adapter）。
 *
 * <p>🔴 <b>本類別刻意沒有 {@code setBalance} 或 {@code reset} 之類的方法</b>：
 * 兩個餘額是兩個科目，只能以 delta 推進（ADR-008 第 1 條保證的機械形式），
 * 而 {@code credit_balance} 永不過期、不因賣家無後續交易而清零（第 2、3 條）
 * ⇒ 提供一個「設定成某值」的入口就等於提供了歸零的工具。
 */
@Repository
public class SellerLogisticsAccountRepository {

    private final SellerLogisticsAccountMapper mapper;

    public SellerLogisticsAccountRepository(SellerLogisticsAccountMapper mapper) {
        this.mapper = mapper;
    }

    /** 確保帳戶存在（冪等），並推進 {@code last_activity_at}。 */
    public void ensureExists(long storeId) {
        mapper.ensureExists(storeId);
    }

    public Optional<SellerLogisticsAccountEntity> find(long storeId) {
        return Optional.ofNullable(mapper.findByStoreId(storeId));
    }

    /**
     * 賣家欠平台的餘額 ± delta。
     *
     * @return {@code false} = 帳戶不存在（呼叫端該先 {@link #ensureExists}）；
     *         🔴 <b>不得</b>把 {@code false} 當成「沒事」——那代表這筆應收沒有落到任何賣家帳上
     */
    public boolean addReceivable(long storeId, long delta) {
        return mapper.addReceivableBalance(storeId, delta) == 1;
    }

    /**
     * 平台欠賣家的貸記 ± delta。
     *
     * <p>🔴 <b>不得被任何排程用來歸零</b>（ADR-008 第 2、3 條）。負 delta 只允許來自
     * {@code LOGISTICS_CREDIT_APPLY}（真的抵扣掉了）；抵扣超額會被
     * {@code chk_sla_credit} 擋下而不是靜默變負。
     */
    public boolean addCredit(long storeId, long delta) {
        return mapper.addCreditBalance(storeId, delta) == 1;
    }
}
