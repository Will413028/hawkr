package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a member updates an address
 * Consumers: order-service (update cached address)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class MemberAddressUpdatedEvent extends BaseEvent {

    private Long memberId;
    private Long addressId;
    private String receiverName;
    private String receiverPhone;
    private String province;
    private String city;
    private String district;
    private String detailAddress;
    private Boolean isDefault;

    public static MemberAddressUpdatedEvent create(Long memberId, Long addressId, String receiverName,
                                                    String receiverPhone, String province, String city,
                                                    String district, String detailAddress, Boolean isDefault) {
        MemberAddressUpdatedEvent event = MemberAddressUpdatedEvent.builder()
                .memberId(memberId)
                .addressId(addressId)
                .receiverName(receiverName)
                .receiverPhone(receiverPhone)
                .province(province)
                .city(city)
                .district(district)
                .detailAddress(detailAddress)
                .isDefault(isDefault)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_MEMBER_ADDRESS_UPDATED, "buyer-service");
        return event;
    }
}
