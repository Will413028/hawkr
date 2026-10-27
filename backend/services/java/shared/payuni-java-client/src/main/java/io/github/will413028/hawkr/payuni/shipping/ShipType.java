package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 物流商類型
 */
public enum ShipType {

    /** 7-ELEVEN */
    SEVEN("1", "7-ELEVEN"),
    /** 大嘴鳥宅配 */
    TAQBIN("2", "大嘴鳥宅配");

    private final String code;
    private final String description;

    ShipType(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() {
        return code;
    }

    public String description() {
        return description;
    }

    public static ShipType fromCode(String code) {
        for (ShipType type : values()) {
            if (type.code.equals(code)) {
                return type;
            }
        }
        return null;
    }
}
