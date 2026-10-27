package io.github.will413028.hawkr.order.exception;

import java.util.List;

/**
 * 庫存不足（含結構化明細）
 */
public class InsufficientStockException extends RuntimeException {

    private final List<Item> items;

    public InsufficientStockException(String message) {
        super(message);
        this.items = List.of();
    }

    public InsufficientStockException(String message, List<Item> items) {
        super(message);
        this.items = items != null ? items : List.of();
    }

    public List<Item> getItems() {
        return items;
    }

    /**
     * 庫存不足的 SKU 明細
     */
    public record Item(String skuId, String requestedQty, String availableQty) {}
}
