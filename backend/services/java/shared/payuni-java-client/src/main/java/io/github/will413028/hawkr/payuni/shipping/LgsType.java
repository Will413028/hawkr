package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 物流通路類型
 */
public enum LgsType {

    /** 超商取貨（店到店） */
    B2C("B2C", "超商取貨"),
    /** 超商寄件（消費者到店） */
    C2C("C2C", "超商寄件"),
    /** 宅配到府 */
    HOME("HOME", "宅配到府"),
    /** 超商退貨（消費者到店退） */
    C2B("C2B", "超商退貨");

    private final String code;
    private final String description;

    LgsType(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() {
        return code;
    }

    public String description() {
        return description;
    }

    public static LgsType fromCode(String code) {
        for (LgsType type : values()) {
            if (type.code.equals(code)) {
                return type;
            }
        }
        return null;
    }
}
