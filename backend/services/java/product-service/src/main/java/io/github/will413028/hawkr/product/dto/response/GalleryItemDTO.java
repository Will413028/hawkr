package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 圖片項目 DTO（gallery 內的結構）
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class GalleryItemDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 資產 ID
     */
    private String assetId;

    /**
     * 儲存路徑
     */
    private String objectKey;
}
