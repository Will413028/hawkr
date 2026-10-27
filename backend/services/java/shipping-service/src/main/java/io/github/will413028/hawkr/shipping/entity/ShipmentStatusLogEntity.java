package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("shipment_status_logs")
public class ShipmentStatusLogEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private Long shipmentId;

    /**
     * 狀態歷程的主體種類：{@code SHIPMENT} / {@code SHIPMENT_BATCH}
     * （值域由 DB CHECK {@code chk_ssl_subject} 守，DEFAULT {@code 'SHIPMENT'}）。
     *
     * <p>🔴 <b>由具名 transition 函式自己填，呼叫端不得傳。</b>
     * 共用表最致命的失效模式是「寫進去的是另一條軸的軌跡」，而那完全無聲
     * （本 repo 的 invoice 側已踩過一次）——讓呼叫端傳值，等於把這個失效模式開放給每一個呼叫點。
     *
     * <p>⚠️ 本波（Foundation）只有 {@code SHIPMENT} 會被寫入；{@code SHIPMENT_BATCH} 待 68 落地。
     * 因此 {@code shipment_id} 這個欄名現在完全誠實，刻意不做 {@code subject_id} 的 rename
     * （rename 會讓 rolling 期間的舊 pod 整筆業務交易 rollback，見 V11 註解 ⑥）。
     */
    private String subjectType;

    /**
     * 推動這次轉移的角色：{@code SYSTEM} / {@code ADMIN} / {@code SELLER} / {@code BUYER} /
     * {@code PROVIDER}（值域由 DB CHECK {@code chk_ssl_actor} 守，DEFAULT {@code 'SYSTEM'}）。
     */
    private String actorType;

    /**
     * 操作者的 {@code account_id}（登入 principal，<b>不是</b> {@code store_id} / {@code buyer_id}）。
     * <p>{@code SYSTEM} / {@code PROVIDER} 觸發時為 NULL。
     */
    private Long actorAccountId;

    /** 觸發這次轉移的 correlation_id（跨服務鏈路追蹤用，選填）。 */
    private String correlationId;

    private String fromStatus;
    private String toStatus;
    private String providerStatus;
    private String providerMessage;
    private String triggerSource;
    private String rawPayload;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
