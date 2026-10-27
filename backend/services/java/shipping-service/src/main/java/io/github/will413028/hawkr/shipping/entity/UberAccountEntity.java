package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Uber Direct 帳號（66 §5.1.1）。
 *
 * <p>三問：寫入＝ADM 設定端點／維運 SQL｜讀取＝{@code UberDirectAdapter} 取 token 與健康檢查｜
 * 推走＝🔴 <b>不刪除</b>，停用一律 {@code enabled=false}。
 *
 * <p>用 {@code enabled} 欄而不是 env 的理由（66 §5.1.1 逐字）：出事那一刻要能由 ADM <b>即時</b>關掉，
 * 而 env 改動要重新部署——我們不想在那一刻等 rollout。
 *
 * <p>🔴 <b>{@code client_secret} 不在這張表</b>：它在 k8s secret {@code uber-direct-credentials}
 * 的 {@code client-secret}（66 §8.5）。任何「順手把 secret 放進來比較好查」的念頭都要擋掉。
 */
@Data
@TableName("uber_account")
public class UberAccountEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * 環境別。⚠️ 66 未定義本欄值域 ⇒ DB 刻意不加 CHECK（加了等於我方發明一個 66 沒有的碼表）。
     */
    private String env;

    /** OAuth2 {@code client_credentials} 的 client_id。🔴 secret 不在本 entity。 */
    private String clientId;

    /** Uber 組織 id（centralized billing 的計費主體，U-H）。 */
    private String organizationId;

    /** 🔴 出事那一刻的煞車：ADM 可即時關閉，不需重新部署。 */
    private Boolean enabled;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
