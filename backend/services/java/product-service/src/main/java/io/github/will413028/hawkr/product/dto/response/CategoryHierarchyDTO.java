package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 三層分類層級 DTO
 * lv3 = product.category 指向的分類，向上查 parent_id 填充 lv2/lv1
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class CategoryHierarchyDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    private Long lv3Id;
    private String lv3Name;
    private Long lv2Id;
    private String lv2Name;
    private Long lv1Id;
    private String lv1Name;
}
