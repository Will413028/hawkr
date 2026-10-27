package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.dto.response.SellerInfoDTO;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import org.springframework.stereotype.Component;

@Component
public class SellerInfoAssembler {

    public SellerInfoDTO from(Long storeId, SellerMeta meta) {
        SellerInfoDTO.SellerInfoDTOBuilder builder = SellerInfoDTO.builder()
                .storeId(storeId);

        if (meta == null) {
            return builder.build();
        }

        if (meta.getSellerName() != null && !meta.getSellerName().isEmpty()) {
            builder.name(meta.getSellerName());
        }
        // 49 §6.6 S9：SellerMeta 內部欄已於批 3 改回 objectKey（logo_key），只填 key，顯示 URL 由 Gateway 組
        if (meta.getLogoKey() != null && !meta.getLogoKey().isEmpty()) {
            builder.logoKey(meta.getLogoKey());
        }

        String city = meta.getCity();
        String district = meta.getDistrict();
        if (city != null && !city.isEmpty()) {
            builder.city(city);
        }
        if (district != null && !district.isEmpty()) {
            builder.district(district);
        }
        if ((city != null && !city.isEmpty()) || (district != null && !district.isEmpty())) {
            builder.address((city != null ? city : "") + (district != null ? district : ""));
        }

        if (meta.getStoreCode() != null && !meta.getStoreCode().isEmpty()) {
            builder.storeCode(meta.getStoreCode());
        }

        return builder.build();
    }
}
