package io.github.will413028.hawkr.order.exception;

import io.github.will413028.hawkr.order.checkout.CheckoutValidator.UnavailableItemInfo;

import java.util.List;

/**
 * 結帳鏈路 row 級不可購買（含下架 / 已售完 / 庫存不足 / lookup 失敗）。
 * <p>
 * 由 {@code CheckoutService.preview} / {@code confirm} 在 resolveItems 後拋出，
 * 由 {@code OrderGrpcService} catch 後組裝 envelope code=4001 +
 * {@code data.unavailable_items[]} response（20 §3.4 / §3.5 / §4.14）。
 */
public class UnavailableItemsException extends RuntimeException {

    private final List<UnavailableItemInfo> items;

    public UnavailableItemsException(List<UnavailableItemInfo> items) {
        super(buildMessage(items));
        this.items = items != null ? items : List.of();
    }

    public List<UnavailableItemInfo> getItems() {
        return items;
    }

    private static String buildMessage(List<UnavailableItemInfo> items) {
        if (items == null || items.isEmpty()) {
            return "商品不可購買";
        }
        StringBuilder sb = new StringBuilder("商品不可購買：");
        for (int i = 0; i < items.size(); i++) {
            if (i > 0) sb.append("、");
            UnavailableItemInfo u = items.get(i);
            sb.append(u.skuName() != null && !u.skuName().isEmpty() ? u.skuName() : "SKU " + u.skuId())
                    .append("（").append(u.reason()).append("）");
        }
        return sb.toString();
    }
}
