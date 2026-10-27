package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Operation names persisted in billing_commands. */
public enum SubscriptionOperation {
    CREATE_SUBSCRIPTION("CREATE_SUBSCRIPTION", "RECURRING_CHARGE"),
    SUSPEND_SUBSCRIPTION("SUSPEND_SUBSCRIPTION", "SUSPEND"),
    RESTART_SUBSCRIPTION("RESTART_SUBSCRIPTION", "RESTART"),
    END_SUBSCRIPTION("END_SUBSCRIPTION", "END");

    private final String value;
    private final String requiredCapability;

    SubscriptionOperation(String value, String requiredCapability) {
        this.value = value;
        this.requiredCapability = requiredCapability;
    }

    public String value() {
        return value;
    }

    public String requiredCapability() {
        return requiredCapability;
    }
}
