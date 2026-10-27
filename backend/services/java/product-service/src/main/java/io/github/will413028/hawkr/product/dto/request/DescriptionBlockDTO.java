package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 描述區塊 DTO
 * 對應 gRPC DescriptionBlock
 *
 * type=text: 文字區塊，使用 runs（或 legacy content）和 styleJson
 * type=table: 表格區塊，使用 tableJson（後端 opaque）
 * type=list: 條列清單區塊，使用 listJson（後端 opaque）
 * type=image: 圖片區塊，使用 stagedToken/assetId, objectKey, width, height, alt
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class DescriptionBlockDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 區塊類型：text / table / list / image
     */
    private String type;

    /**
     * 區塊識別碼
     */
    private String id;

    // === text 類型欄位 ===

    /**
     * 文字內容（type=text 時使用）
     */
    private String content;

    /**
     * 樣式 JSON 字串（type=text 時使用）
     * 格式: {"fontSize":18,"fontWeight":"bold","color":"#333","alignment":"center"}
     */
    private String styleJson;

    // === image 類型欄位 ===

    /**
     * 暫存 Token（新增圖片時傳入，Finalize 前）
     */
    private String stagedToken;

    /**
     * 資產 ID（既有圖片，Finalize 後）
     */
    private String assetId;

    /**
     * OSS 儲存路徑
     */
    private String objectKey;

    /**
     * 完整 URL（回應時由後端組合，不落盤）
     */
    private String url;

    /**
     * 圖片寬度
     */
    private Integer width;

    /**
     * 圖片高度
     */
    private Integer height;

    /**
     * 替代文字
     */
    private String alt;

    // === inline-run 模型欄位（02 §3.6）===

    /**
     * 行內 runs（type=text 時使用；取代 legacy content）
     */
    private java.util.List<InlineRunDTO> runs;

    /**
     * 表格資料 JSON 字串（type=table 時使用；cells 內含 runs，後端 opaque）
     */
    private String tableJson;

    /**
     * 清單資料 JSON 字串（type=list 時使用；items 內含 runs，後端 opaque）
     */
    private String listJson;
}
