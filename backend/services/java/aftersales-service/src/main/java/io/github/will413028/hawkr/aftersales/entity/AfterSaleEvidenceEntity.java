package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("aftersale_evidence")
public class AfterSaleEvidenceEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String caseNo;

    private Long assetId;

    private String objectKey;

    private Long uploadedBy;

    private String evidenceType;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
