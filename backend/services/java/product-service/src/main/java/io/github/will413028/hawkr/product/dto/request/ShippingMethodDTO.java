package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 運送方式 DTO
 * 使用 common-core Enum value
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ShippingMethodDTO {

    /** ShippingMethod Enum value（如 3001、3002） */
    private String type;

    /** ShippingTemperature Enum value（如 3201、3202、3203） */
    private String temperature;

    /** 免運門檻（新台幣元，數字字串，訂單金額 >= 此值則免運） */
    private String freeThreshold;

    /** ShippingFeePayer Enum value（如 3101、3102） */
    private String feePayer;
}
