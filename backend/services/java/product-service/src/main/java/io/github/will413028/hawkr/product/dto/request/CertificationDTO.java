package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 商品認證 DTO
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class CertificationDTO {

    /**
     * 認證名稱
     */
    private String name;

    /**
     * 認證編號
     */
    private String certNo;
}
