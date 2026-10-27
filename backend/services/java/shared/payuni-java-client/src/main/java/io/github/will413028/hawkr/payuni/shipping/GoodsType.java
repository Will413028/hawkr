package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 商品溫層類型
 */
public enum GoodsType {

    /** 常溫 */
    NORMAL("1", "常溫"),
    /** 冷凍 */
    FROZEN("2", "冷凍"),
    /** 冷藏 */
    REFRIGERATED("3", "冷藏");

    private final String code;
    private final String description;

    GoodsType(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() {
        return code;
    }

    public String description() {
        return description;
    }

    public static GoodsType fromCode(String code) {
        for (GoodsType type : values()) {
            if (type.code.equals(code)) {
                return type;
            }
        }
        return null;
    }
}
