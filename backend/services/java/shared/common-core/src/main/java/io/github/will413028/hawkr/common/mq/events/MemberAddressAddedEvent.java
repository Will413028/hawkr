package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a member adds a new address
 * Consumers: order-service (cache default address)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class MemberAddressAddedEvent extends BaseEvent {

    private Long memberId;
    private Long addressId;
    private String receiverName;
    private String receiverPhone;
    private String province;
    private String city;
    private String district;
    private String detailAddress;
    private Boolean isDefault;

    public static MemberAddressAddedEvent create(Long memberId, Long addressId, String receiverName,
                                                  String receiverPhone, String province, String city,
                                                  String district, String detailAddress, Boolean isDefault) {
        MemberAddressAddedEvent event = MemberAddressAddedEvent.builder()
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
        event.initializeEvent(MqConstants.EVENT_TYPE_MEMBER_ADDRESS_ADDED, "buyer-service");
        return event;
    }
}
