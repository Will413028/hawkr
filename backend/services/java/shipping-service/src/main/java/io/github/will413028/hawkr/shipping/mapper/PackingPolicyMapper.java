package io.github.will413028.hawkr.shipping.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.shipping.entity.PackingPolicyEntity;

/**
 * 分箱政策（版本化，66 §5.1.8）。
 *
 * <p>本波（Stage 8B W1，資料層）刻意<b>不加任何自訂 {@code @Select}</b>：
 * 狀態轉換的 CAS 述詞、租約 claim、TTL 掃描都屬 W2，寫在這裡會變成沒有呼叫端的死碼。
 * <p>🔴 W2 新增自訂 SQL 時：回傳 {@code PackingPolicyEntity} 的查詢必須 SELECT <b>全部</b>非 transient 欄位
 * （partial projection 禁令，10 檔 §11.3）；只要部分欄位就回 record／DTO 放
 * {@code entity/projection/}。
 */
public interface PackingPolicyMapper extends BaseMapper<PackingPolicyEntity> {
}
