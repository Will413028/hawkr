package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 代行批次寫入的 before 快照（56 §7.2.8 步驟 ②）。
 *
 * <p>不是 ProductDetailDTO 的子集投影，而是獨立的唯讀結構：前四欄服務狀態轉換的前置判斷
 * （逐項 CAS 要的 version、部分成功回應要的 status／name），{@code detail} 服務稽核的
 * before_snapshot。
 *
 * <p>{@code detail} 只在呼叫端明確要求時填充（{@code withDetail=true}）。批次寫入失敗項
 * 回填現行 version 的路徑只需要 version，讓它一併付掉三次額外查詢是白花的。
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductSnapshotDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    private String productId;

    /** false = 不存在或不屬於該 store（兩者不區分，避免探測） */
    private boolean found;

    private Long version;

    private String status;

    private String name;

    /**
     * 稽核 before_snapshot 的完整投影素材（56 §6.1.11）。
     *
     * <p>Gateway 拿它跑與單筆路徑同一支投影函式，因此這裡必須與單筆讀取路徑填一樣的東西
     * ——{@code toDetailDTO} ＋ hashtags ＋ 三層分類，缺哪一項，稽核就會把該欄記成空值。
     * 少填 hashtags 的形狀在單筆路徑上出過事（見 ProductService 更新狀態那一段的註解）。
     *
     * <p>⚠️ <b>刻意不含 seller／reviews</b>：投影用不到，而填 seller 要跨服務呼叫
     * seller-service，逐項付一次會讓批次的第一跳退化成 N 次 RPC。
     */
    private ProductDetailDTO detail;
}
