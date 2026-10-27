package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsCostFactMapper;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * provider 成本事實的 repository（thin adapter）。
 *
 * <p>70 §3.1.4 durable join 的<b>成本側</b>。它只做兩件事：寫一筆事實、依 leg 查回來。
 *
 * <p>🔴 <b>刻意沒有 UPDATE 入口</b>：事實不會被改寫。同一條 leg 之後出現的費用走
 * true-up（§3.1.3），改的是 receivable 的 {@code actual_cost}，不是回頭改這一列。
 * 開一條改寫入口等於讓「歷史成本」變成可變的，而對帳正是拿它當基準。
 */
@Repository
public class LogisticsCostFactRepository {

    private final LogisticsCostFactMapper mapper;

    public LogisticsCostFactRepository(LogisticsCostFactMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * @return {@code true} ＝ 這次真的寫了；{@code false} ＝ 已存在（重投／重複回報）。
     *         🔴 {@code false} <b>不是錯誤</b>，呼叫端應 ACK。
     */
    public boolean insertIfAbsent(LogisticsCostFactEntity entity) {
        return mapper.insertIfAbsent(entity) == 1;
    }

    /** 依 provider leg 取回成本事實；至多一筆（{@code uq_costfact_delivery_ref}）。 */
    public Optional<LogisticsCostFactEntity> findByProviderDeliveryRef(String providerDeliveryRef) {
        if (providerDeliveryRef == null || providerDeliveryRef.isBlank()) {
            return Optional.empty();
        }
        return Optional.ofNullable(mapper.findByProviderDeliveryRef(providerDeliveryRef));
    }
}
