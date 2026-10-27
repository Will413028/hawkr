package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("shipping_webhook_dedup")
public class ShippingWebhookDedupEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String eventKey;
    private String shipTradeNo;
    private String processStatus;
    private LocalDateTime processedAt;
}
