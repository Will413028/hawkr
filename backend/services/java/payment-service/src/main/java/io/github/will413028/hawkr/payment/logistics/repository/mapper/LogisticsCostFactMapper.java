package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * {@code logistics_cost_facts}（V16）的 mapper。
 *
 * <p>🔴 <b>本表只有兩種操作：寫入一筆事實、依 leg 查回來。</b>
 * 沒有 UPDATE、沒有狀態轉換、沒有 claim——它是 append-only 的事實表，不是狀態機。
 * 同一條 leg 之後出現的費用走 true-up（70 §3.1.3），改的是 receivable 的
 * {@code actual_cost}，不是回頭改這一列。
 */
@Mapper
public interface LogisticsCostFactMapper extends BaseMapper<LogisticsCostFactEntity> {

    /**
     * 寫入一筆成本事實，以<b>兩道 UNIQUE</b> 去重。
     *
     * <p>🔴 {@code ON CONFLICT DO NOTHING} <b>不指名 conflict target</b>：本表有兩道 UNIQUE
     * （{@code uq_costfact_delivery_ref} 與 {@code uq_costfact_event}），指名任一個都會讓
     * 另一個的衝突變成<b>例外</b>而不是「什麼都不做」。
     * ⚠️ 而 PostgreSQL 交易內任一句 SQL 失敗 ⇒ 整個交易 aborted（{@code 25P02}），
     * catch 得到例外但下一句 SELECT 會炸（10 檔 §11.1）⇒ 那不是可行的替代做法。
     *
     * <p>兩道 UNIQUE 擋的是<b>不同</b>的事，缺一不可：
     * <ul>
     *   <li>{@code uq_costfact_event} —— 同一則事件被 broker 重投（L1）</li>
     *   <li>{@code uq_costfact_delivery_ref} —— 同一條 leg 被<b>不同事件</b>重複回報成本
     *       （例如 producer 重試時產生了新的 {@code event_id}）</li>
     * </ul>
     *
     * @return 1 ＝ 新建；0 ＝ 已存在（呼叫端據此 ACK，<b>不是錯誤</b>）
     */
    @Insert("""
            INSERT INTO logistics_cost_facts
                (id, provider, provider_delivery_ref, store_id, order_no,
                 shipment_id, estimated_cost, occurred_at, event_id)
            VALUES
                (#{id}, #{provider}, #{providerDeliveryRef}, #{storeId}, #{orderNo},
                 #{shipmentId}, #{estimatedCost}, #{occurredAt}, #{eventId})
            ON CONFLICT DO NOTHING
            """)
    int insertIfAbsent(LogisticsCostFactEntity entity);

    /**
     * 依 provider leg 取回成本事實。
     *
     * <p>🔴 <b>不帶 provider 條件</b>：{@code uq_costfact_delivery_ref} 是<b>單欄</b> UNIQUE，
     * 與 {@code logistics_receivables.uq_logrecv_delivery_ref} 同口徑 ⇒ 至多一筆。
     * 兩張表對「什麼是同一條 leg」必須是同一個答案，否則 join 會對到錯的列。
     */
    @Select("""
            SELECT * FROM logistics_cost_facts
             WHERE provider_delivery_ref = #{providerDeliveryRef}
            """)
    LogisticsCostFactEntity findByProviderDeliveryRef(
            @Param("providerDeliveryRef") String providerDeliveryRef);
}
