package io.github.will413028.hawkr.order.exception;

import java.util.List;

/**
 * 結帳金額變動（Preview → Confirm 間商品價格/運費/優惠券發生變化）
 */
public class CheckoutPriceChangedException extends RuntimeException {

    private final long oldTotalAmount;
    private final long newTotalAmount;
    private final long newSubtotal;
    private final long newShippingFee;
    private final long newDiscount;
    private final List<String> changeReasons;

    public CheckoutPriceChangedException(String message,
                                          long oldTotalAmount, long newTotalAmount,
                                          long newSubtotal, long newShippingFee, long newDiscount,
                                          List<String> changeReasons) {
        super(message);
        this.oldTotalAmount = oldTotalAmount;
        this.newTotalAmount = newTotalAmount;
        this.newSubtotal = newSubtotal;
        this.newShippingFee = newShippingFee;
        this.newDiscount = newDiscount;
        this.changeReasons = changeReasons;
    }

    public long getOldTotalAmount() { return oldTotalAmount; }
    public long getNewTotalAmount() { return newTotalAmount; }
    public long getNewSubtotal() { return newSubtotal; }
    public long getNewShippingFee() { return newShippingFee; }
    public long getNewDiscount() { return newDiscount; }
    public List<String> getChangeReasons() { return changeReasons; }
}
