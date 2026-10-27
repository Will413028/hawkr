package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 配送時段標記
 */
public enum DeliveryTimeTag {

    /** 13 時前 */
    BEFORE_13("01", "13時前"),
    /** 14-18 時 */
    AFTERNOON("02", "14-18時"),
    /** 不指定 */
    ANY("04", "不指定");

    private final String code;
    private final String description;

    DeliveryTimeTag(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() {
        return code;
    }

    public String description() {
        return description;
    }

    public static DeliveryTimeTag fromCode(String code) {
        for (DeliveryTimeTag tag : values()) {
            if (tag.code.equals(code)) {
                return tag;
            }
        }
        return null;
    }
}
