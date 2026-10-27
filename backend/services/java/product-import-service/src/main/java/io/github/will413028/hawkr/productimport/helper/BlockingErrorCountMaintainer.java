package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

/**
 * 維護 {@code product_import_item.blocking_error_count} 反正規化欄位（40 號設計書 §3.11）。
 *
 * <p>規則（docs/開發規範/後端服務責任邊界規範.md）：寫入路徑同事務維護，禁止 reconciliation job 重算。
 *
 * <ul>
 *   <li>寫入 ERROR severity 的 product_import_item_error → 同事務 +1</li>
 *   <li>刪除 / 編輯化解 ERROR → 同事務 -1</li>
 *   <li>{@code commit eligible filter} 用 {@code = 0}</li>
 * </ul>
 */
@Component
public class BlockingErrorCountMaintainer {

    private final ProductImportItemMapper itemMapper;

    public BlockingErrorCountMaintainer(ProductImportItemMapper itemMapper) {
        this.itemMapper = itemMapper;
    }

    /**
     * 同事務 +1。必須在「寫入 ERROR severity 的 product_import_item_error」同一事務內呼叫。
     *
     * @return 更新後的 blocking_error_count
     */
    @Transactional(propagation = Propagation.MANDATORY)
    public Integer incrementError(Long itemId) {
        return itemMapper.adjustBlockingErrorCount(itemId, 1);
    }

    /**
     * 同事務 -1。必須在「刪除 / 化解 ERROR」同一事務內呼叫。
     *
     * @return 更新後的 blocking_error_count（不會小於 0；DB constraint 在 PR2 可加）
     */
    @Transactional(propagation = Propagation.MANDATORY)
    public Integer decrementError(Long itemId) {
        return itemMapper.adjustBlockingErrorCount(itemId, -1);
    }
}
