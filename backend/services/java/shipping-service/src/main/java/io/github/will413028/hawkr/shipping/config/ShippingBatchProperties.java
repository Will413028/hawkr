package io.github.will413028.hawkr.shipping.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "hawkr.shipping.batch")
public class ShippingBatchProperties {

    private int maxSize = 50;

    private int homeLabelConcurrency = 10;
}
