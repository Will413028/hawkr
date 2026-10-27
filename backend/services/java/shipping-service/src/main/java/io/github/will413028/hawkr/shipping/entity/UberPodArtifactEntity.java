package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Uber 送達憑證 POD（66 §5.1.7／§7.8）。
 *
 * <p>三問：寫入＝delivery 進 {@code delivered} 時插入待抓列 ＋ P2 抓取排程回填｜
 * 讀取＝賣家／買家 POD 端點（S-U5）與爭議處理｜
 * 推走＝P2 抓到後寫 {@link #fetchedAt}（離開候選集合）；逾 7 天窗口（T-12）建 case
 * {@code UBER_POD_MISSING} 並在 timeline 留一行「POD 不可得，原因 X」。
 *
 * <p>🔴 <b>INV-U5 是「每筆 {@code delivered} 在 7 天內有 ≥1 筆」而不是「三種都要有」</b>——
 * {@code leave_at_door} 只會有 {@code picture}，要求齊全會讓每一單都被判成缺證據。
 * ⚠️ 反過來，「一筆都沒有」是真的異常：Uber 對 {@code delivered} 一定至少有一種 POD。
 *
 * <p>⚠️ 保存期限：Uber 的照片只留 7 天、簽名 30 天，而爭議窗口是 D+7 ＋ 人工裁決
 * ⇒ 差額可能為負 ⇒ 必須<b>主動抓回自存</b>（66 §0.5）。
 */
@Data
@TableName("uber_pod_artifacts")
public class UberPodArtifactEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /** 對應 {@code uber_deliveries.delivery_id}。與 {@link #artifactType} 合成 UNIQUE（66 §7.9 擋重複上傳）。 */
    private String deliveryId;

    /**
     * {@code picture}｜{@code signature}｜{@code pincode}（Uber 原生值照抄，DB CHECK 守）。
     * <p>{@code leave_at_door} ⇒ 只有 {@code picture}；需簽收 ⇒ {@code signature}（可能另有 {@code picture}）；
     * 需 PIN ⇒ {@code pincode}。
     */
    private String artifactType;

    private Long assetId;

    /**
     * 🔴 欄名叫 {@code _key}、存的就是 key（<b>不是 URL</b>）——{@code secure/} 前綴。
     * <p>⚠️ 用 {@code public/} 前綴的後果是陷阱 A6：收件地址與門口實景可被任何拿到 objectKey 的人取得。
     * <p>⚠️ objectKey 本身是<b>憑證等級</b>的值（66 §8.6）⇒ log 必須遮罩。
     */
    private String objectKey;

    private String pincodeValue;

    /** 抓到的時間。非 null ⇒ 離開 {@code idx_pod_fetch_lease} 的候選集合。 */
    private LocalDateTime fetchedAt;

    private Integer fetchAttempts;

    /**
     * 🔴 <b>租約到期時間，不是「已抓過」旗標</b>（66 §5.1.7）。
     * <p>NULL 或 {@code <= NOW()} 表示可搶佔；租約長度 10 分鐘（T-11：下界 &gt; POD deadline 20s×3
     * ＋ media 20s ≈80s；上界 &lt; 7 天窗口）。
     * <p>永久旗標會讓 pod 在抓取中途崩潰的那一筆永遠卡死且無告警。
     */
    private LocalDateTime fetchLeaseUntil;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
