package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity.GovVerifyPayload;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * 商品認證申請 Repository（19 號設計書《農業認證驗證》§4.1／§6.3／§6.6）。
 *
 * <p>三個必須守住的機制：
 * <ol>
 *   <li><b>{@code INSERT ... ON CONFLICT DO NOTHING}</b>（§6.3 Step 5）：靠 partial unique index
 *       {@code uk_pca_pending} 做併發冪等。<b>回傳 rowsAffected 必須被呼叫端檢查</b> ——
 *       「不同號碼併發」時 DO NOTHING 是<b>靜默資料遺失</b>（無列、無錯），
 *       商品路徑要落進 {@code certificationWarnings[]}、專用端點要回 409。</li>
 *   <li><b>CAS UPDATE</b>（§6.6）：自訂 {@code @Update} 不受 OptimisticLockerInnerInterceptor 保護，
 *       因此每一句都手寫 {@code version = version + 1} 並在 WHERE 帶 {@code version = #{expectedVersion}}。
 *       {@code rowsAffected = 0} 的處置依呼叫端而異（ADM 拋 409 讓交易 rollback；
 *       背景查驗放棄不重試；同步自動核准跳過該筆）。</li>
 *   <li><b>①→② 順序</b>（§6.6）：先把同 type 舊 APPROVED 轉 SUPERSEDED，再把本筆轉 APPROVED。
 *       {@code uk_pca_active} 是逐句即時檢查的 partial unique index，順序顛倒會撞 23505 讓整個交易 abort。</li>
 * </ol>
 *
 * <p>⚠️ 所有回傳 {@code ProductCertificationApplicationEntity} 的查詢一律 {@code SELECT pca.*}
 * （Java 微服務開發規範 §9.2 partial projection 禁令）；需要商品名之類的外部欄位時由 Service
 * 另行以 {@code ProductRepository.findPublicationBriefByIdsIncludingDeleted（ADM 路徑；不濾可見性，見該方法 javadoc）} 補齊，不在這裡 JOIN 進 Entity。
 * 自訂 SQL 皆帶 {@code @ResultMap}，否則 {@code gov_verify_payload} 的 TypeHandler 不會生效。
 */
@Mapper
public interface ProductCertificationApplicationRepository
        extends BaseMapper<ProductCertificationApplicationEntity> {

    String RESULT_MAP = "mybatis-plus_ProductCertificationApplicationEntity";

    // ==================== §6.3 同步規則：Step 1 / 2 / 3 ====================

    /**
     * 依 (product, type, certNo, statuses) 查申請 —— §6.3 Step 1（APPROVED 命中即 no-op）、
     * Step 2（SUBMITTED 命中即 no-op）、Step 3（REJECTED／REVOKED 命中 ⇒ requiresManualReview = true）共用。
     *
     * <p>🔴 Step 1 必須在最前面：新列一律以 SUBMITTED 插入，同 (type, certNo) 已 APPROVED 時
     * 不會觸發任何唯一索引衝突 ⇒ 少了這一步，每存一次商品就替每個已核准認證多建一筆 SUBMITTED。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        <script>
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.certification_type = #{certificationType}
          AND pca.certificate_no = #{certificateNo}
          AND pca.status IN
          <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
        ORDER BY pca.id DESC
        </script>
        """)
    List<ProductCertificationApplicationEntity> findByProductTypeCertNoAndStatuses(
            @Param("productId") Long productId,
            @Param("certificationType") Integer certificationType,
            @Param("certificateNo") String certificateNo,
            @Param("statuses") List<String> statuses);

    // ==================== §6.3 Step 4：SUPERSEDE 同 type 的異號待審 ====================

    /**
     * 鎖定同 (product, type) 但號碼不同的待審申請 —— §6.3 Step 4 的第一步。
     *
     * <p>先查再更新（而非一句 UPDATE 帶走）的理由：<b>每一列被 SUPERSEDE 都要寫一筆 review_event</b>，
     * 包含併發競態造成的退場（T2 把 T1 剛建立的申請轉 SUPERSEDED）。不先取出就寫不出稽核。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.certification_type = #{certificationType}
          AND pca.status = 'SUBMITTED'
          AND pca.certificate_no <> #{certificateNo}
        FOR UPDATE
        """)
    List<ProductCertificationApplicationEntity> lockSubmittedByProductAndTypeExceptCertNo(
            @Param("productId") Long productId,
            @Param("certificationType") Integer certificationType,
            @Param("certificateNo") String certificateNo);

    /**
     * 把一筆標成「只能人工裁決」（§8.5.1 業者名稱歸屬未通過時）。
     *
     * <p><b>刻意不帶 version、不 bump version</b>：這不是業務狀態轉換，是「把它排除在自動核准之外」
     * 的一次性標記，不該讓賣家或 ADM 手上的 {@code expectedVersion} 平白失效。
     * {@code WHERE requires_manual_review = false} 讓它天然冪等（重複呼叫回 0）。
     *
     * <p>不寫 {@code updated_at}：本表其餘自訂 {@code @Update} 皆不寫（自訂 SQL 不走
     * MetaObjectHandler），這裡跟隨既有慣例，不在同一張表上製造第二種行為。
     *
     * @return 實際更新的列數（0 = 本來就已標記）
     */
    @Update("""
        UPDATE product_certification_application
        SET requires_manual_review = TRUE
        WHERE id = #{id} AND requires_manual_review = FALSE
        """)
    int markRequiresManualReview(@Param("id") Long id);

    /**
     * 把指定的申請轉成終態（SUPERSEDED / WITHDRAWN）。
     *
     * <p>WHERE 帶 {@code status = #{expectedStatus}} 讓它對重複執行是冪等的；
     * 自訂 SQL 不受樂觀鎖攔截器保護，故手寫 {@code version = version + 1}。
     *
     * <p>（2026-08-29：本段 javadoc 原本錯置在 {@code markRequiresManualReview} 上方——
     * 兩段 javadoc 疊在一起時 Java 只認第二段，等於本方法一直沒有註解。）
     *
     * @return 實際轉移的列數（呼叫端據此逐列寫 review_event）
     */
    @Update("""
        <script>
        UPDATE product_certification_application
        SET status = #{toStatus},
            reviewed_at = #{reviewedAt},
            version = version + 1
        WHERE status = #{expectedStatus}
          AND id IN
          <foreach collection="ids" item="id" open="(" separator="," close=")">#{id}</foreach>
        </script>
        """)
    int transitionToTerminalStatus(@Param("ids") List<Long> ids,
                                   @Param("expectedStatus") String expectedStatus,
                                   @Param("toStatus") String toStatus,
                                   @Param("reviewedAt") LocalDateTime reviewedAt);

    // ==================== §6.3 Step 5：ON CONFLICT DO NOTHING ====================

    /**
     * 建立申請 —— <b>一律以 {@code status = 'SUBMITTED'} 插入</b>（§6.3 Step 5）。
     *
     * <p>🔴 不得直接插 APPROVED：{@code uk_pca_active} 的謂詞是 {@code status='APPROVED'}，
     * 同 (product, type) 已有別的號碼是 APPROVED 時直接以 APPROVED 插入會撞唯一索引，
     * 而 {@code ON CONFLICT DO NOTHING} 會<b>靜默丟棄新申請</b>。
     * 先插 SUBMITTED 再走 §6.6 的 ①→② 兩步，是唯一同時滿足「換號主路徑」與「不撞 23505」的順序。
     *
     * <p>⚠️ {@code DO NOTHING} 刻意<b>不指名 conflict target</b>：對任何唯一約束衝突都靜默跳過
     * （只有 {@code DO UPDATE} 才強制指名）。PostgreSQL 的交易在任一句 SQL 失敗後即 aborted，
     * 「讓 DB 拋唯一鍵錯再 catch」在此結構上行不通。
     *
     * @return 1 = 已建立；<b>0 = 未建立</b>（併發衝突或同 type 已有待審），呼叫端必須處理
     */
    @Insert("""
        INSERT INTO product_certification_application
            (id, product_id, store_id, submitted_by_account_id, certification_type, certificate_no,
             status, source, requires_manual_review, gov_verify_state, gov_verify_payload,
             gov_verified_at, version, submitted_at, created_at, updated_at)
        VALUES
            (#{id}, #{productId}, #{storeId}, #{submittedByAccountId}, #{certificationType},
             #{certificateNo}, 'SUBMITTED', #{source}, #{requiresManualReview}, #{govVerifyState},
             #{govVerifyPayload, typeHandler=io.github.will413028.hawkr.product.config.typehandler.GovVerifyPayloadTypeHandler},
             #{govVerifiedAt}, 0, #{submittedAt}, #{submittedAt}, #{submittedAt})
        ON CONFLICT DO NOTHING
        """)
    int insertSubmittedIfAbsent(
            @Param("id") Long id,
            @Param("productId") Long productId,
            @Param("storeId") Long storeId,
            @Param("submittedByAccountId") Long submittedByAccountId,
            @Param("certificationType") Integer certificationType,
            @Param("certificateNo") String certificateNo,
            @Param("source") String source,
            @Param("requiresManualReview") boolean requiresManualReview,
            @Param("govVerifyState") String govVerifyState,
            @Param("govVerifyPayload") GovVerifyPayload govVerifyPayload,
            @Param("govVerifiedAt") LocalDateTime govVerifiedAt,
            @Param("submittedAt") LocalDateTime submittedAt);

    // ==================== §6.6 核准 transaction ====================

    /** 鎖定單筆申請（§6.6 第一步：lock application ＋ product）。 */
    @ResultMap(RESULT_MAP)
    @Select("SELECT pca.* FROM product_certification_application pca WHERE pca.id = #{id} FOR UPDATE")
    Optional<ProductCertificationApplicationEntity> findByIdForUpdate(@Param("id") Long id);

    /** 不加鎖的單筆查詢（讀取面向／背景查驗取當下 version 用）。 */
    @ResultMap(RESULT_MAP)
    @Select("SELECT pca.* FROM product_certification_application pca WHERE pca.id = #{id}")
    Optional<ProductCertificationApplicationEntity> findById(@Param("id") Long id);

    /**
     * 同 (product, type) 現行有效那筆（§6.6 ①的對象、§9.2 的 {@code currentApproved}）。
     * {@code uk_pca_active} 保證至多一列。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.certification_type = #{certificationType}
          AND pca.status = 'APPROVED'
        """)
    Optional<ProductCertificationApplicationEntity> findApprovedByProductAndType(
            @Param("productId") Long productId,
            @Param("certificationType") Integer certificationType);

    /**
     * 該商品當前所有 APPROVED 申請 —— {@code rebuildProjection(productId)} 的<b>唯一輸入集合</b>（§7.2）。
     * 排序固定，讓投影 JSONB 對同一集合是決定性的（diff／稽核不會因排序抖動誤判變更）。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.status = 'APPROVED'
        ORDER BY pca.certification_type, pca.id
        """)
    List<ProductCertificationApplicationEntity> findApprovedByProductId(@Param("productId") Long productId);

    /**
     * CAS 狀態轉移（§6.6 ①②、ADM 裁決、背景自動核准共用）。
     *
     * @param expectedVersion 人工＝使用者帶的 expectedVersion；系統＝讀取當下的 version
     * @param allowedFromStatus 目前狀態必須等於它（狀態機守門，避免對終態重複裁決）
     * @return 1 = 成功；<b>0 = CAS 失敗</b>（版本已變或狀態已轉出），處置依呼叫端而異
     */
    @Update("""
        UPDATE product_certification_application
        SET status = #{toStatus},
            reviewed_at = #{reviewedAt},
            reviewed_by_account_id = #{reviewedByAccountId},
            reason_code = #{reasonCode},
            public_reason = #{publicReason},
            internal_note = COALESCE(#{internalNote}, internal_note),
            version = version + 1
        WHERE id = #{id}
          AND version = #{expectedVersion}
          AND status = #{allowedFromStatus}
        """)
    int casTransitionStatus(@Param("id") Long id,
                            @Param("expectedVersion") Long expectedVersion,
                            @Param("allowedFromStatus") String allowedFromStatus,
                            @Param("toStatus") String toStatus,
                            @Param("reviewedAt") LocalDateTime reviewedAt,
                            @Param("reviewedByAccountId") Long reviewedByAccountId,
                            @Param("reasonCode") String reasonCode,
                            @Param("publicReason") String publicReason,
                            @Param("internalNote") String internalNote);

    // ==================== §8.3 背景查驗回寫 ====================

    /**
     * 回寫政府查驗結果（背景查驗 consumer ／ ADM「重新查驗」）。
     *
     * <p>只動查驗欄位、不動 status —— 狀態轉移一律走 {@link #casTransitionStatus}。
     * 仍帶 version CAS：期間若已被人工裁決，{@code rowsAffected = 0} ⇒ 放棄，不重試、不報錯。
     *
     * @return 1 = 已回寫；0 = 期間已被裁決
     */
    @Update("""
        UPDATE product_certification_application
        SET gov_verify_state = #{govVerifyState},
            gov_verify_payload = #{govVerifyPayload,
                typeHandler=io.github.will413028.hawkr.product.config.typehandler.GovVerifyPayloadTypeHandler},
            gov_verified_at = #{govVerifiedAt},
            version = version + 1
        WHERE id = #{id}
          AND version = #{expectedVersion}
        """)
    int casUpdateGovVerifyResult(@Param("id") Long id,
                                 @Param("expectedVersion") Long expectedVersion,
                                 @Param("govVerifyState") String govVerifyState,
                                 @Param("govVerifyPayload") GovVerifyPayload govVerifyPayload,
                                 @Param("govVerifiedAt") LocalDateTime govVerifiedAt);

    // ==================== §6.4 全量取代：該 type 完全沒送 ====================

    /**
     * 該商品目前有 APPROVED 但 type 不在本次清單內的申請（§6.4 → WITHDRAWN 的對象）。
     *
     * <p>🔴 呼叫端必須先過 §6.3 Step 0 的 {@code effectiveUpdate} 閘門：
     * {@code updateCertifications || !certifications.isEmpty()}。少了它，任何部分更新都會讓
     * 「本次送出的 type 集合」變成空集合 ⇒ 該商品全部 APPROVED 一次轉 WITHDRAWN ⇒ 法規標章消失，
     * 而且 HTTP 200、稽核有列、不發推播 —— 零訊號。
     *
     * <p>粒度是 {@code certification_type} 而非 (type, certNo)：換號時舊號碼必須保持公開到新號碼核准為止。
     * {@code SUBMITTED} 刻意不在本查詢內（待審是進行中的請求，只能由明確 withdraw 撤銷）。
     *
     * @param submittedTypes 本次送出的 type 集合；<b>空集合代表「明確清空」</b>（updateCertifications=true + 空陣列）
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        <script>
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.status = 'APPROVED'
        <if test="submittedTypes != null and submittedTypes.size() > 0">
          AND pca.certification_type NOT IN
          <foreach collection="submittedTypes" item="t" open="(" separator="," close=")">#{t}</foreach>
        </if>
        FOR UPDATE
        </script>
        """)
    List<ProductCertificationApplicationEntity> lockApprovedByProductExcludingTypes(
            @Param("productId") Long productId,
            @Param("submittedTypes") List<Integer> submittedTypes);

    /**
     * 鎖住某商品**全部** {@code SUBMITTED} 的申請（#2820：商品軟刪時的 cascade）。
     *
     * <p>與 {@link #lockApprovedByProductExcludingTypes} 是兩支而不是一支參數化的方法：
     * 那支是「全量取代時把沒送的 type 退場」，要排除本次送出的 type；本支是「商品沒了，
     * 賣家對它的一切宣稱都停止」，**不排除任何 type**。
     *
     * <p>🔴 刻意只鎖 {@code SUBMITTED}，不碰 {@code APPROVED}：{@code REVOKED} 是平台對
     * 假標章的**唯一治理手段**（政府 API 只驗號碼存在、不驗歸屬），自動清掉已核准的申請
     * 等於給賣家一條「刪商品即抹除盜用證號的治理紀錄」的路。已軟刪商品沒有公開投影，
     * 標章本來就不會顯示，留著只影響 ADM 的稽核視圖 —— 那正是要留的東西。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.status = 'SUBMITTED'
        FOR UPDATE
        """)
    List<ProductCertificationApplicationEntity> lockSubmittedByProductId(
            @Param("productId") Long productId);

    // ==================== §9.1 賣家查詢 ====================

    /** 賣家查自己商品的申請（§9.1）；{@code status} 為 null 時不篩。 */
    @ResultMap(RESULT_MAP)
    @Select("""
        <script>
        SELECT pca.* FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.store_id = #{storeId}
        <if test="status != null and status != ''">
          AND pca.status = #{status}
        </if>
        ORDER BY pca.submitted_at DESC, pca.id DESC
        LIMIT #{limit} OFFSET #{offset}
        </script>
        """)
    List<ProductCertificationApplicationEntity> findByProductAndStore(
            @Param("productId") Long productId,
            @Param("storeId") Long storeId,
            @Param("status") String status,
            @Param("limit") int limit,
            @Param("offset") int offset);

    @Select("""
        <script>
        SELECT COUNT(*) FROM product_certification_application pca
        WHERE pca.product_id = #{productId}
          AND pca.store_id = #{storeId}
        <if test="status != null and status != ''">
          AND pca.status = #{status}
        </if>
        </script>
        """)
    long countByProductAndStore(@Param("productId") Long productId,
                                @Param("storeId") Long storeId,
                                @Param("status") String status);

    // ==================== §9.2 Admin 佇列 ====================

    /**
     * ADM 審核佇列（§9.2）。keyword 同時比對商品名與證書編號 ⇒ JOIN product，
     * 但<b>只 SELECT pca.*</b>（partial projection 禁令）；商品名由 Service 另行補齊。
     * 排序用 {@code idx_pca_queue}（status, submitted_at）。
     */
    @ResultMap(RESULT_MAP)
    @Select("""
        <script>
        SELECT pca.* FROM product_certification_application pca
        <if test="keyword != null and keyword != ''">
        JOIN product p ON p.id = pca.product_id
        </if>
        <where>
          <if test="status != null and status != ''">AND pca.status = #{status}</if>
          <if test="certificationType != null">AND pca.certification_type = #{certificationType}</if>
          <if test="storeId != null">AND pca.store_id = #{storeId}</if>
          <if test="submittedFrom != null">AND pca.submitted_at &gt;= #{submittedFrom}</if>
          <if test="submittedTo != null">AND pca.submitted_at &lt;= #{submittedTo}</if>
          <if test="keyword != null and keyword != ''">
            AND (p.name ILIKE '%' || #{keyword} || '%' OR pca.certificate_no ILIKE '%' || #{keyword} || '%')
          </if>
        </where>
        ORDER BY pca.submitted_at DESC, pca.id DESC
        LIMIT #{limit} OFFSET #{offset}
        </script>
        """)
    List<ProductCertificationApplicationEntity> findForAdmin(
            @Param("status") String status,
            @Param("certificationType") Integer certificationType,
            @Param("storeId") Long storeId,
            @Param("keyword") String keyword,
            @Param("submittedFrom") LocalDateTime submittedFrom,
            @Param("submittedTo") LocalDateTime submittedTo,
            @Param("limit") int limit,
            @Param("offset") int offset);

    @Select("""
        <script>
        SELECT COUNT(*) FROM product_certification_application pca
        <if test="keyword != null and keyword != ''">
        JOIN product p ON p.id = pca.product_id
        </if>
        <where>
          <if test="status != null and status != ''">AND pca.status = #{status}</if>
          <if test="certificationType != null">AND pca.certification_type = #{certificationType}</if>
          <if test="storeId != null">AND pca.store_id = #{storeId}</if>
          <if test="submittedFrom != null">AND pca.submitted_at &gt;= #{submittedFrom}</if>
          <if test="submittedTo != null">AND pca.submitted_at &lt;= #{submittedTo}</if>
          <if test="keyword != null and keyword != ''">
            AND (p.name ILIKE '%' || #{keyword} || '%' OR pca.certificate_no ILIKE '%' || #{keyword} || '%')
          </if>
        </where>
        </script>
        """)
    long countForAdmin(@Param("status") String status,
                       @Param("certificationType") Integer certificationType,
                       @Param("storeId") Long storeId,
                       @Param("keyword") String keyword,
                       @Param("submittedFrom") LocalDateTime submittedFrom,
                       @Param("submittedTo") LocalDateTime submittedTo);
}
