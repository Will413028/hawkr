package io.github.will413028.hawkr.shipping.provider.uber;

import com.baomidou.mybatisplus.core.toolkit.Wrappers;
import io.github.will413028.hawkr.shipping.entity.UberStoreMappingEntity;
import io.github.will413028.hawkr.shipping.mapper.UberStoreMappingMapper;

import java.util.Optional;

/**
 * 賣家 {@code store_id} → Uber {@code customer_id} ／ {@code external_store_id} 的解析
 * （66 §5.1.3 {@code uber_store_mapping}）。
 *
 * <h2>為什麼獨立成一個元件，而不是讓 adapter 直接拿 mapper</h2>
 * ① adapter 的單元測試不必背上 MyBatis；② 「哪一列算可用」這個述詞只有一份
 * ——{@code active=true AND blocked=false}（INV-U6 由 DB 的 CHECK 守，但讀取側也要一致）。
 *
 * <p>🔴 <b>解析不到不得回預設值</b>：Uber 的 {@code customer_id} 是路徑參數（總覽 §1.3），
 * 猜一個或留空會讓請求打到別人的 customer 或直接 404，而兩者在 log 上都長得像「Uber 掛了」。
 */
public class UberStoreResolver {

    private final UberStoreMappingMapper storeMappingMapper;

    public UberStoreResolver(UberStoreMappingMapper storeMappingMapper) {
        this.storeMappingMapper = storeMappingMapper;
    }

    /**
     * 取這個賣家目前可用的 Uber 門市對應。
     *
     * @return empty 代表尚未建店、已關閉，或被 Uber 封鎖（{@code customer_blocked} 的事後反推）。
     *         呼叫端必須以 {@code UBER_STORE_NOT_MAPPED} 語意擋下，<b>不得</b>當成「Uber 暫時有問題」
     */
    public Optional<UberStoreMappingEntity> activeMapping(long sellerStoreId) {
        // ⚠️ selectList + 取第一筆而不是 selectOne：一個賣家可有多個 external_store_id
        //    （66 §7.4 道 1「同賣家 ≠ 同取件地址」），selectOne 命中多列會直接拋。
        return storeMappingMapper.selectList(Wrappers.<UberStoreMappingEntity>lambdaQuery()
                        .eq(UberStoreMappingEntity::getSellerStoreId, sellerStoreId)
                        .eq(UberStoreMappingEntity::getActive, true)
                        .eq(UberStoreMappingEntity::getBlocked, false)
                        .orderByAsc(UberStoreMappingEntity::getId))
                .stream()
                .findFirst();
    }
}
