package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;
import java.util.List;

/**
 * 商品列表結果 DTO
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductListResultDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 商品列表
     */
    private List<ProductListItemDTO> products;

    /**
     * 總筆數
     */
    private long total;

    /**
     * 當前頁碼
     */
    private int page;

    /**
     * 每頁筆數
     */
    private int pageSize;
}
