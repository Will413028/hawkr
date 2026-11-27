package io.github.will413028.hawkr.order;

import java.util.UUID;

/** 測試用的固定 UUID：商品與 SKU 的 ID 是 catalog 的 UUID，測試以小整數編號產生可讀、可重現的值。 */
public final class TestIds {

    private TestIds() {
    }

    /** 同一個編號永遠得到同一個 UUID；不同編號得到不同 UUID。 */
    public static UUID id(long n) {
        return new UUID(0x0192f3a47b1c7d2eL, 0x8f3a000000000000L | n);
    }
}
