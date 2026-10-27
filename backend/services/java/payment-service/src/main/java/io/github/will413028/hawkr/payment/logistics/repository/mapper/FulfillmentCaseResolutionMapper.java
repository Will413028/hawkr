package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code fulfillment_case_resolutions} 的 mapper。
 *
 * <p>🔴 entity 有 TypeHandler 欄位（{@code impact_preview} JSONB）⇒ 自訂 {@code @Select} 必須掛
 * {@code @ResultMap("mybatis-plus_FulfillmentCaseResolutionEntity")}。
 */
@Mapper
public interface FulfillmentCaseResolutionMapper extends BaseMapper<FulfillmentCaseResolutionEntity> {

    /**
     * 建立提案。
     *
     * <p>🔴 {@code ON CONFLICT} 指名 {@code uq_fcr_open_per_case} 的 partial 述詞
     * （{@code (case_id) WHERE status = 'AWAITING_REVIEW'}）：同一張案件同時只能有一個待覆核提案。
     * 不帶 {@code WHERE} 推斷不到那個 partial index。
     *
     * <p>🔴🔴 <b>述詞裡的狀態值必須是字面值，不得用 {@code #{}} 綁定</b>（與
     * {@code FulfillmentExceptionCaseMapper.insertIfNoOpenDuplicate} 同一個實測結論）：
     * 參數化的 {@code ON CONFLICT … WHERE} 只在前 5 次 EXECUTE（custom plan）成立，
     * 第 6 次切成 generic plan 後永久 ERROR 42P10。pgjdbc 的 {@code prepareThreshold}
     * 預設同樣是 5 ⇒ <b>本機與 CI 都會是綠的，上線後第 6 次才炸</b>。
     * <p>⚠️ 這個字面值是索引述詞的複製（schema 事實），漂移由
     * {@code LogisticsSchemaContractTest} 逐字對拍 V9 守住。
     *
     * <p>🔴 {@code caseCategory} 必須來自<b>剛剛 {@code FOR UPDATE} 鎖住的那一列</b>，
     * 不得由呼叫端自行指定：複合 FK {@code fk_fcr_case_category} 會以 23503 擋下謊報，
     * 而 {@code chk_fcr_responsibility_type_binding} 再雙向綁定它與 {@code resolutionType}。
     *
     * @return 1 = 新建；0 = 該案件已有待覆核提案
     */
    @Insert("""
            INSERT INTO fulfillment_case_resolutions (
                id, case_id, case_category, resolution_type, proposed_responsibility_cause,
                reason, impact_preview, impact_computed_at,
                requires_review, status, proposed_by_account_id
            ) VALUES (
                #{e.id}, #{e.caseId}, #{e.caseCategory}, #{e.resolutionType},
                #{e.proposedResponsibilityCause,jdbcType=VARCHAR}, #{e.reason},
                #{e.impactPreview,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler},
                #{e.impactComputedAt}, #{e.requiresReview}, #{e.status}, #{e.proposedByAccountId}
            )
            ON CONFLICT (case_id) WHERE status = 'AWAITING_REVIEW'
            DO NOTHING
            """)
    int insertIfNoPendingProposal(@Param("e") FulfillmentCaseResolutionEntity entity);

    @ResultMap("mybatis-plus_FulfillmentCaseResolutionEntity")
    @Select("SELECT * FROM fulfillment_case_resolutions WHERE id = #{id}")
    FulfillmentCaseResolutionEntity findById(@Param("id") Long id);

    /**
     * 覆核前讀回 maker 的提案，<b>帶列鎖</b>（V13）。
     *
     * <p>🔴 <b>為什麼不能用 {@link #findById}</b>：覆核要「讀出提案成因 → 據以結案」，
     * 讀與寫之間若無鎖，另一個交易可以在這個窗口把同一列改掉
     * ⇒ 結案用的成因與最終落地的那一列不一致。
     *
     * <p>🔴 <b>為什麼不能只靠 case 的 {@code FOR UPDATE}</b>：那道鎖在
     * {@code fulfillment_exception_cases}，不及於本表的列。今天所有寫入路徑都先鎖 case
     * 只是<b>慣例</b>，而慣例會在下一次重構被繞過而沒有人發現。
     *
     * <p>⚠️ 鎖的取得順序固定為 <b>case → resolution</b>（與 {@code propose}／{@code reject}
     * 一致），不得反向，否則兩條路徑互等形成 deadlock。
     */
    @ResultMap("mybatis-plus_FulfillmentCaseResolutionEntity")
    @Select("SELECT * FROM fulfillment_case_resolutions WHERE id = #{id} FOR UPDATE")
    FulfillmentCaseResolutionEntity findByIdForUpdate(@Param("id") Long id);

    @ResultMap("mybatis-plus_FulfillmentCaseResolutionEntity")
    @Select("""
            SELECT * FROM fulfillment_case_resolutions
            WHERE case_id = #{caseId}
            ORDER BY proposed_at DESC
            """)
    List<FulfillmentCaseResolutionEntity> listByCase(@Param("caseId") Long caseId);

    /**
     * 覆核決議（CAS）。
     *
     * <p>🔴 {@code chk_fcr_two_person} 在 DB 層擋「覆核者 ＝ 提案者」；本句另外在述詞裡
     * <b>再擋一次</b>（{@code proposed_by_account_id &lt;&gt; #{reviewedBy}}）。兩者不重複：
     * DB 那道會丟 23514（呼叫端拿到的是 500），述詞這道讓它變成 rows=0 的正常業務結果，
     * 呼叫端可以回一個有意義的業務碼。
     *
     * <p>🔴 {@code chk_fcr_decided_pair} 是雙向等式（已決 ⇔ 有 {@code reviewed_at}）⇒
     * 狀態與時間必須同一句寫。
     *
     * @return 1 = 決議成功；0 = 已不在 {@code AWAITING_REVIEW}，或覆核者就是提案者
     */
    @Update("""
            UPDATE fulfillment_case_resolutions
            SET status = #{toStatus},
                reviewed_by_account_id = #{reviewedByAccountId},
                review_comment = #{reviewComment,jdbcType=VARCHAR},
                reviewed_at = #{reviewedAt},
                updated_at = NOW()
            WHERE id = #{id}
              AND status = #{fromStatus}
              AND proposed_by_account_id <> #{reviewedByAccountId}
            """)
    int decide(@Param("id") Long id,
               @Param("fromStatus") String fromStatus,
               @Param("toStatus") String toStatus,
               @Param("reviewedByAccountId") Long reviewedByAccountId,
               @Param("reviewComment") String reviewComment,
               @Param("reviewedAt") LocalDateTime reviewedAt);

    /**
     * <b>責任判定</b>提案的覆核決議（CAS，V12）。
     *
     * <p>🔴 與 {@link #decide} 的差別是述詞多一個 {@code resolution_type}
     * ——那是 36 §3.3 N16 的 Atomic Boundary ① <b>逐字要求</b>，且不可省：
     * 少了它，一個非責任型別的提案也會被這條路徑推成 {@code APPROVED}，
     * 而呼叫端接著會依「resolution 已 APPROVED」把案件結掉並發 N16
     * ⇒ {@code fulfillment.responsibility.finalized} 的「唯一產生點」就不再唯一。
     *
     * <p>⚠️ DB 的 {@code chk_fcr_responsibility_type_binding} 已擋住「責任案件掛非責任型別」
     * 這個組合，所以本述詞在今天是<b>第二道</b>。兩道方向一致、不互相取代：
     * DB 那道是建立時擋（23514 ⇒ 500），本述詞讓它在覆核時變成 {@code rows=0} 的正常業務結果。
     *
     * <p>🔴 {@code proposed_by_account_id <> #{reviewedByAccountId}} 沿用 {@link #decide} 的
     * two-person 述詞：{@code APPROVED} 的雙人保證是「不向買家退款」這個決定的程序要件。
     *
     * @return 1 = 決議成功；0 = 已不在 {@code fromStatus}、型別不符、或覆核者就是提案者
     */
    @Update("""
            UPDATE fulfillment_case_resolutions
            SET status = #{toStatus},
                reviewed_by_account_id = #{reviewedByAccountId},
                review_comment = #{reviewComment,jdbcType=VARCHAR},
                reviewed_at = #{reviewedAt},
                updated_at = NOW()
            WHERE id = #{id}
              AND case_id = #{caseId}
              AND status = #{fromStatus}
              AND resolution_type = #{resolutionType}
              AND proposed_by_account_id <> #{reviewedByAccountId}
            """)
    int decideResponsibility(@Param("id") Long id,
                             @Param("caseId") Long caseId,
                             @Param("fromStatus") String fromStatus,
                             @Param("toStatus") String toStatus,
                             @Param("resolutionType") String resolutionType,
                             @Param("reviewedByAccountId") Long reviewedByAccountId,
                             @Param("reviewComment") String reviewComment,
                             @Param("reviewedAt") LocalDateTime reviewedAt);

    /**
     * 一批案件各自的處置提案（manual review 的載體）。
     *
     * <p>🔴 本 entity 有 JSONB 欄位（{@code impact_preview}）⇒ 必須掛 {@code @ResultMap}。
     */
    @ResultMap("mybatis-plus_FulfillmentCaseResolutionEntity")
    @Select("""
            <script>
            SELECT * FROM fulfillment_case_resolutions
            WHERE case_id IN
              <foreach collection="caseIds" item="id" open="(" separator="," close=")">#{id}</foreach>
            ORDER BY case_id, proposed_at DESC
            </script>
            """)
    List<FulfillmentCaseResolutionEntity> resolutionsByCaseIds(@Param("caseIds") List<Long> caseIds);
}
