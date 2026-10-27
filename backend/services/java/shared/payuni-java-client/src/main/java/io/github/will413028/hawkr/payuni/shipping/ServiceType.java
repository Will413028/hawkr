package io.github.will413028.hawkr.payuni.shipping;

/**
 * PayUni 物流服務類型
 */
public enum ServiceType {

    /** 取貨付款 */
    CASH_ON_DELIVERY("1", "取貨付款"),
    /** 取貨不付款 */
    NO_CASH_ON_DELIVERY("3", "取貨不付款");

    private final String code;
    private final String description;

    ServiceType(String code, String description) {
        this.code = code;
        this.description = description;
    }

    public String code() {
        return code;
    }

    public String description() {
        return description;
    }

    public static ServiceType fromCode(String code) {
        for (ServiceType type : values()) {
            if (type.code.equals(code)) {
                return type;
            }
        }
        return null;
    }
}
