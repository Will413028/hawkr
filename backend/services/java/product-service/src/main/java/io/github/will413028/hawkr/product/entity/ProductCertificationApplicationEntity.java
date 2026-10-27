package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.FieldStrategy;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import com.baomidou.mybatisplus.annotation.Version;
import io.github.will413028.hawkr.product.config.typehandler.GovVerifyPayloadTypeHandler;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 商品農產品認證申請與審核 Entity（19 號設計書《農業認證驗證》§4.1，Flyway V18）。
 *
 * <p><b>本表是認證的權威來源</b>；{@code product.certifications} JSONB 只是
 * 「由當前所有 {@code APPROVED} 申請重算」的 approved-only 公開投影（§4.3 / §7.2）。
 * 核心不變式：<b>只有 APPROVED 會進入公開投影</b> —— 賣家填了什麼 ≠ 平台認可什麼。
 *
 * <p>值域一律走 CodeMeta（{@code CodeMetaValidator} 在寫入前把關）：
 * {@code status} = {@link io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus}、
 * {@code source} = {@link io.github.will413028.hawkr.common.utils.code.ProductCertificationSource}、
 * {@code govVerifyState} = {@link io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState}、
 * {@code reasonCode} = {@link io.github.will413028.hawkr.common.utils.code.ProductCertificationRejectReason}、
 * {@code certificationType} = {@link io.github.will413028.hawkr.common.utils.code.CertificationType} 的 value（3701–3706）。
 * DB 端另有三個 CHECK 作為 fail-closed 第二道（V18 檔頭已標明它是 CodeMeta 的刻意拷貝）。
 */
@Data
@TableName(value = "product_certification_application", autoResultMap = true)
public class ProductCertificationApplicationEntity implements Serializable {

    private static final long serialVersionUID = 1L;

    /**
     * 申請 ID（TSID）。IdType.INPUT ⇒ 由 {@code TsidUtil.generate()} 產生後帶入。
     *
     * <p>V18 存量遷移的列由純 SQL 產生（raw unix epoch ms &lt;&lt; 22 + row_number），
     * 值域刻意落在應用層 TSID 之上，結構上不會碰撞（理由見 V18 檔內註解）。
     */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** 所屬商品 ID */
    private Long productId;

    /**
     * 賣家 store 代理鍵（擁有權欄位，與 {@code product.store_id} 對齊）。
     * 這是<b>營運鍵</b>，不是登入 principal（見 {@link #submittedByAccountId}）。
     */
    private Long storeId;

    /**
     * 送出者登入 principal（賣家本人或代行 ADM）。
     *
     * <p><b>nullable 且不得以 storeId 頂替</b>：{@code source = LEGACY_MIGRATION} 的列必為 null
     * （migration 沒有送出者）。兩者僅在「首任 grandfather」時恰好相等，拿 storeId 頂替會
     * 讓 dev 與 fixture 全綠、只有 re-key 過的店家錯而且沒有任何告警（CLAUDE.md §6）。
     */
    private Long submittedByAccountId;

    /**
     * 認證類型（{@code CertificationType.value()} 的數值形式，3701–3706）。
     *
     * <p>DB 為 INTEGER、CodeMeta value 為字串 ⇒ 驗證時以 {@code String.valueOf()} 餵
     * {@code CodeMetaValidator}，對外 JSON 一律走 {@code formatInt}。
     */
    private Integer certificationType;

    /** 證書編號（只 trim，保留前導 0，不轉數字） */
    private String certificateNo;

    /** 審核狀態（ProductCertificationStatus；DB DEFAULT 'SUBMITTED'） */
    private String status;

    /** 申請來源（ProductCertificationSource：SELLER／PRODUCT_IMPORT／DELEGATED_ADMIN／SEED／LEGACY_MIGRATION） */
    private String source;

    /**
     * true ⇒ <b>永不自動核准</b>（§6.3 Step 3）。
     * 曾被 REJECTED／REVOKED 的號碼重送時設為 true，避免政府查驗剛好通過就繞過 ADM 裁決。
     */
    private Boolean requiresManualReview;

    /** 政府查驗狀態（ProductCertificationGovVerifyState；DB DEFAULT 'PENDING'） */
    private String govVerifyState;

    /**
     * 政府查驗結構化結果（§8.3 {@code GovVerifyResult} 的修剪版），ADM 判斷依據。
     * <b>Admin-only</b>：不得出現在任何賣家／公開 DTO。
     */
    @TableField(typeHandler = GovVerifyPayloadTypeHandler.class)
    private GovVerifyPayload govVerifyPayload;

    /** 最後一次查驗完成時間（UTC） */
    private LocalDateTime govVerifiedAt;

    /**
     * 樂觀鎖版本。
     *
     * <p>{@code updateById} / {@code update(entity, wrapper)} 由 common-core 全域註冊的
     * OptimisticLockerInnerInterceptor 自動 CAS + 1；<b>{@code @Update} 自訂 SQL 不受攔截器保護，
     * 需逐句手寫 {@code version = version + 1}</b>（Repository 內的 CAS 語句皆已手寫）。
     */
    @Version
    private Long version;

    /** 進入 SUBMITTED 的時間；ADM 佇列排序鍵（DB DEFAULT NOW()） */
    private LocalDateTime submittedAt;

    /** 進入終態的時間 */
    private LocalDateTime reviewedAt;

    /** 裁決者 account id；自動核准為 null（actor 由 review_event 的 SYSTEM 標示） */
    private Long reviewedByAccountId;

    /** 駁回／撤銷原因碼（ProductCertificationRejectReason） */
    private String reasonCode;

    /** 會展示給賣家的原因說明 */
    private String publicReason;

    /** <b>僅 ADM 可見</b>，不得出現在任何賣家／公開 DTO */
    private String internalNote;

    /** 建立時間（UPDATE 時不可修改） */
    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    /** 更新時間 */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * {@code gov_verify_payload} JSONB 的持久化形狀（19 §8.3 {@code GovVerifyResult} 的修剪版）。
     *
     * <p>欄位與 proto {@code GovVerifyResult} 一一對應（key camelCase），
     * 讓 Converter 能逐欄映射而不必在轉換層重新定義形狀。
     * 服務層的查驗結果 record 負責填進來；本類別只描述「存進 DB 的是什麼」。
     */
    @Data
    public static class GovVerifyPayload implements Serializable {
        private static final long serialVersionUID = 1L;

        /** CertificationType value（字串形式，3701–3706） */
        private String certType;
        /** 查驗的證書編號 */
        private String certNo;
        /** 查驗時間（ISO 8601 UTC） */
        private String queriedAt;
        /** 政府端是否查得到該編號 */
        private Boolean found;
        /** VALID / EXPIRED / TERMINATED / SUSPENDED / NOT_FOUND / UNKNOWN */
        private String validity;
        /** 政府端原始狀態字串（3701 才有） */
        private String statusRaw;
        /** 效期（3703 沒有） */
        private String expiresOn;
        /** 證書持有者名稱 */
        private String holderName;
        /** 證書持有者地址 */
        private String holderAddress;
        /** 發證機構 */
        private String issuerName;
        /** 品項摘要 */
        private String productSummary;
        /** 命中筆數（供 ADM 判斷是否為整廠清單，如 3703 的工廠編號） */
        private Integer matchedCount;
        /** 已修剪的原始回應片段（§8.4 防線 3：我方截斷 payload） */
        private String rawExcerptJson;
    }
}
