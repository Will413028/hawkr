package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 商品圖片 DTO
 * 支援暫存 token 與既有資產兩種模式
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductImageDTO {

    /**
     * 前端排序或暫時識別用（可選）
     */
    private String id;

    /**
     * 新增圖片用：只在「建立/新增」階段出現
     */
    private String stagedToken;

    /**
     * 既有圖片用：保留/更新時使用
     */
    private String assetId;

    /**
     * 判斷是否為新增圖片（有 stagedToken）
     */
    public boolean isNewImage() {
        return stagedToken != null && !stagedToken.isBlank();
    }

    /**
     * 判斷是否為既有圖片（有 assetId）
     */
    public boolean isExistingImage() {
        return assetId != null && !assetId.isBlank();
    }
}
