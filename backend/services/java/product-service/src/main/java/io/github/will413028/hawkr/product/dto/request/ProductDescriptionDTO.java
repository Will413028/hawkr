package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;
import java.util.List;

/**
 * 商品描述 DTO（支援圖文混排）
 * 對應 gRPC ProductDescription
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductDescriptionDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 版本號
     */
    private String version;

    /**
     * 區塊列表
     */
    private List<DescriptionBlockDTO> blocks;
}
