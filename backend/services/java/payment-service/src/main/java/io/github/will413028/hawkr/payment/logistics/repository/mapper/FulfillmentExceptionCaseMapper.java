package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code fulfillment_exception_cases} 的 mapper。
 *
 * <p>🔴 本 entity 有 TypeHandler 欄位（兩個 JSONB）⇒ 每一支自訂 {@code @Select} 都必須掛
 * {@code @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")}，否則 JSONB 欄位取回時
 * 不走 handler（10 檔 §11.1；precedent：{@code OutboxEventMapper.findPendingEvents}）。
 *
 * <p>🔴 <b>狀態轉換一律 CAS</b>，且 allowed-from 由<b>呼叫端</b>傳入：本表的狀態機目前<b>沒有</b>
 * 對應的 {@code *Rules} helper（{@code ReceivableStatusRules} 只管應收那一條軸），
 * 所以這裡不在 SQL 裡硬寫一份清單、也不憑空造第二個碼表——把 allowed-from 留在呼叫端，
 * 等它的規則有了唯一權威再收斂進去。⚠️ 這是<b>誠實標註的缺口</b>，不是設計：
 * 在那之前，「這條邊合不合法」沒有機械保證，只有 CAS 保證「不會從錯的狀態跳過去」。
 */
@Mapper
public interface FulfillmentExceptionCaseMapper extends BaseMapper<FulfillmentExceptionCaseEntity> {

    /** {@code 'EX' + yyyyMMdd + LPAD(nextval('exception_case_no_seq'),8,'0')}。 */
    @Select("""
            SELECT 'EX' || to_char(NOW() AT TIME ZONE 'UTC', 'YYYYMMDD')
                || LPAD(nextval('exception_case_no_seq')::text, 8, '0')
            """)
    String nextCaseNo();

    /**
     * 建案，以 {@code dedup_key} 去重。
     *
     * <p>🔴 {@code ON CONFLICT} 必須<b>指名 partial index 的述詞</b>
     * （{@code (dedup_key) WHERE status IN ('OPEN','IN_PROGRESS')}）：{@code uq_exc_open_dedup}
     * 是 partial unique index，不帶 {@code WHERE} 的 {@code ON CONFLICT (dedup_key)} 推斷不到它，
     * PostgreSQL 會直接報 42P10（沒有相符的推斷目標），而不是靜默走別條路。
     *
     * <p>🔴🔴 <b>那個述詞裡的狀態值必須是字面值，不得用 {@code #{}} 綁定</b>——這是實測出來的，
     * 不是風格偏好。PostgreSQL 17.9 實測（一次性 DB，V1..V9）：
     * <pre>
     * ON CONFLICT (dedup_key) WHERE status IN ($2, $4)
     *   → 前 5 次 EXECUTE 正常（custom plan 會把參數值代進去，述詞蘊含證得出來）
     *   → 第 6 次起永久 ERROR 42P10 "there is no unique or exclusion constraint
     *      matching the ON CONFLICT specification"（切成 generic plan 後證不出來）
     *   → force_generic_plan 下第 1 次就炸
     * </pre>
     * ⚠️ 這個失效模式<b>最危險的地方是它會通過所有淺測</b>：pgjdbc 的
     * {@code prepareThreshold} 預設也是 5，於是本機跑幾次、CI 跑幾次都是綠的，
     * 上線後同一條連線打到第 6 次才炸，而且從此不會再好。
     * <p>字面值版在 {@code force_generic_plan} 下實測正常（新 key INSERT 1、重複 INSERT 0、
     * 案件結案後同 key 可再開一張）。
     * <p>⚠️ 這兩個字面值是<b>索引述詞的複製</b>（schema 事實），不是業務碼表——
     * 漂移由 {@code LogisticsSchemaContractTest} 逐字對拍 V9 的 {@code uq_exc_open_dedup} 守住。
     *
     * @return 1 = 新建；0 = 已有未結案的同 dedup_key 案件（呼叫端據此判定「已經開過了」）
     */
    @Insert("""
            INSERT INTO fulfillment_exception_cases (
                id, case_no, source_service, category, severity, subject_type, subject_ref,
                store_id, status, dedup_key, order_no, shipment_id, shipping_method,
                detail, proposed_action_codes, occurred_at
            ) VALUES (
                #{e.id}, #{e.caseNo}, #{e.sourceService}, #{e.category}, #{e.severity},
                #{e.subjectType}, #{e.subjectRef}, #{e.storeId,jdbcType=BIGINT}, #{e.status},
                #{e.dedupKey},
                #{e.orderNo,jdbcType=VARCHAR}, #{e.shipmentId,jdbcType=BIGINT},
                #{e.shippingMethod,jdbcType=VARCHAR},
                #{e.detail,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler},
                #{e.proposedActionCodes,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler},
                #{e.occurredAt}
            )
            ON CONFLICT (dedup_key) WHERE status IN ('OPEN', 'IN_PROGRESS')
            DO NOTHING
            """)
    int insertIfNoOpenDuplicate(@Param("e") FulfillmentExceptionCaseEntity entity);

    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("SELECT * FROM fulfillment_exception_cases WHERE id = #{id}")
    FulfillmentExceptionCaseEntity findById(@Param("id") Long id);

    /**
     * 建立<b>責任判定</b>案件（V12）。
     *
     * <p>🔴 <b>與 {@link #insertIfNoOpenDuplicate} 的差別只有一處，而那一處是必要的</b>：
     * 本句的 {@code ON CONFLICT} <b>不指名 conflict target</b>。
     *
     * <p>理由（65 的 <b>G4</b>）：V12 之後責任判定案件同時受兩條唯一約束拘束——
     * {@code uq_exc_open_dedup}（dedup_key，僅未結案之間）與 {@code uq_exc_responsibility_subject}
     * （subject，<b>lifetime</b>）。指名 {@code (dedup_key) WHERE …} 的那一支只認得前者，
     * 撞到後者時<b>不會</b>走 {@code DO NOTHING} 而是拋 {@code 23505}；而 PostgreSQL 交易內
     * 任一句 SQL 失敗 ⇒ 整個交易 aborted（{@code 25P02}），之後所有指令一律被拒直到 ROLLBACK
     * ⇒ 症狀不是「重複案件被安靜擋下」，而是<b>整個建案交易 500</b>。
     *
     * <p>不指名 target 的 {@code DO NOTHING} 對<b>任何</b>唯一約束衝突都靜默跳過
     * （10 檔 §11.1：{@code DO NOTHING} 可以不指名，只有 {@code DO UPDATE} 必須指名），
     * 這正是「同一張表同時存在兩個唯一約束」時唯一安全的寫法。
     *
     * <p>🔴 本方法是 <b>N17 的 L2 冪等</b>（36 §3.3）：只有真的寫進一列的那一次交易才 INSERT outbox。
     * 回 0 ⇒ 該 leg 已經有責任案件 ⇒ <b>不得</b>發第二筆事件。
     *
     * @return 1 = 新建；0 = 該 leg 已有責任判定案件（lifetime，含已結案者）
     */
    @Insert("""
            INSERT INTO fulfillment_exception_cases (
                id, case_no, source_service, category, severity, subject_type, subject_ref,
                store_id, status, dedup_key, order_no, shipment_id, shipping_method,
                detail, proposed_action_codes, occurred_at
            ) VALUES (
                #{e.id}, #{e.caseNo}, #{e.sourceService}, #{e.category}, #{e.severity},
                #{e.subjectType}, #{e.subjectRef}, #{e.storeId,jdbcType=BIGINT}, #{e.status},
                #{e.dedupKey},
                #{e.orderNo,jdbcType=VARCHAR}, #{e.shipmentId,jdbcType=BIGINT},
                #{e.shippingMethod,jdbcType=VARCHAR},
                #{e.detail,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler},
                #{e.proposedActionCodes,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler},
                #{e.occurredAt}
            )
            ON CONFLICT DO NOTHING
            """)
    int insertResponsibilityCaseIfAbsent(@Param("e") FulfillmentExceptionCaseEntity entity);

    /**
     * 取一列並鎖住（{@code SELECT … FOR UPDATE}）。
     *
     * <p>🔴 <b>proposal 與 finalization 都必須先走這一句</b>（65 ⓪、36 §3.3 Atomic
     * Boundary ⓪）。它把同一張 case 上的提案與定案序列化——少了它，兩個並行的 finalization
     * 各自讀到同一份狀態、各自往下走，而<b>每一步單獨看都是成功的</b>。
     *
     * <p>🔴 明令廢除 snapshot-based finality：不得以「掃一次、看到這張 case 上沒有
     * {@code AWAITING_REVIEW} 的提案」當判準——那是 read-then-act，兩個並行交易會同時讀到
     * 「沒有」然後同時往下走，而且不會有任何錯誤訊息。
     *
     * <p>⚠️ 鎖的範圍是<b>同一張 case</b> 不是同一個 subject——跨 subject 的責任判定本來就該並行；
     * 「同一個 subject 只能有一張責任 case」由 {@code uq_exc_responsibility_subject} 承擔，不靠鎖。
     *
     * <p>⚠️ 必須在交易內呼叫；不在交易內時 PostgreSQL 的 {@code FOR UPDATE} 會在語句結束時
     * 立刻釋放鎖 ⇒ 序列化保證消失而<b>沒有任何錯誤</b>。呼叫端以 {@code @Transactional} 保證。
     *
     * <p>🔴 <b>誠實標註：今天沒有任何測試「只靠它」才會綠</b>（2026-09-16 WP2A 實測）。
     * 把本句的 {@code FOR UPDATE} 拿掉之後，責任判定的並發測試組 5/5 仍然全綠——
     * 因為每一條競態都先被別的東西接住了：
     * <ul>
     *   <li>並行定案 → resolution 的 CAS（{@code status = 'AWAITING_REVIEW'}）只讓一個成功</li>
     *   <li>並行提案 → {@code uq_fcr_open_per_case} 只讓一筆待覆核提案存在</li>
     *   <li>定案 vs 提案 → 上面兩者合起來，READ COMMITTED 下兩種順序都收斂到一致結果</li>
     * </ul>
     * <b>這不是「可以拿掉」的理由，而是「拿掉之後不會有人發現」的警告。</b>
     * 它仍然必須留著：① 65 ⓪ 與 36 §3.3 Atomic Boundary ⓪ 逐字要求
     * ② 上述三道接住它的東西<b>都是可以被改掉的</b>——任何未來的路徑若不走 CAS
     * （例如「讀出來、在 Java 裡判斷、再寫回去」），序列化就只剩這一句
     * ③ 它讓「先鎖後判」成為這條路徑的形狀，而不是每個 caller 各自記得。
     * <p>⇒ 要為它建立真正的鑑別力，需要一個<b>不受 CAS 保護</b>的競態場景；
     * 在那之前，本段就是它的誠實邊界（{@code .claude/rules/06-no-small-tails.md} §3.10）。
     */
    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("SELECT * FROM fulfillment_exception_cases WHERE id = #{id} FOR UPDATE")
    FulfillmentExceptionCaseEntity findByIdForUpdate(@Param("id") Long id);

    /**
     * 依 {@code (category, subject_type, subject_ref)} 取責任判定案件（lifetime 只會有一張）。
     *
     * <p>用於建案撞到 {@code DO NOTHING} 之後回讀既有案件——{@code ON CONFLICT DO NOTHING}
     * 不回傳衝突的那一列，呼叫端要拿到它只能再查一次。
     */
    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("""
            SELECT * FROM fulfillment_exception_cases
            WHERE category = #{category} AND subject_type = #{subjectType} AND subject_ref = #{subjectRef}
            """)
    FulfillmentExceptionCaseEntity findResponsibilityCase(@Param("category") String category,
                                                          @Param("subjectType") String subjectType,
                                                          @Param("subjectRef") String subjectRef);

    /**
     * 依對外業務單號 {@code case_no} 取單一案件（ADM 詳情與 maker-checker 的入口）。
     *
     * <p>🔴 <b>不帶 {@code FOR UPDATE}</b>：它服務唯讀詳情，以及 maker-checker 端點在
     * <b>進入交易之前</b>把 {@code case_no} 換成 {@code case_id}／{@code order_no} 的那一步。
     * 真正的序列化仍然由 {@link #findByIdForUpdate} 在交易內做（65 ⓪）——
     * 在這裡加鎖會把鎖的持有時間拉長到涵蓋一次跨服務 gRPC（讀 order 的退款基準），
     * 那正是 36 §3.3.1 明令禁止的「交易內做 enrich」。
     *
     * <p>⚠️ 因此呼叫端拿到的 {@code case_id} 是<b>快照</b>。這是安全的：{@code case_no}
     * 由 {@code uq_exc_no} 保證唯一且**永不改變**，{@code order_no} 由 V14 凍結在建案當下、
     * 全 repo 無更新路徑 ⇒ 交易內重讀到的必然是同一列同一個 order_no。
     * 會變的只有 {@code status}，而那正是交易內 CAS 述詞要判的東西。
     */
    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("SELECT * FROM fulfillment_exception_cases WHERE case_no = #{caseNo}")
    FulfillmentExceptionCaseEntity findByCaseNo(@Param("caseNo") String caseNo);

    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("""
            <script>
            SELECT * FROM fulfillment_exception_cases
            WHERE dedup_key = #{dedupKey}
              AND status IN
              <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
            ORDER BY created_at DESC
            LIMIT 1
            </script>
            """)
    FulfillmentExceptionCaseEntity findOpenByDedupKey(@Param("dedupKey") String dedupKey,
                                                      @Param("statuses") List<String> statuses);

    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("""
            <script>
            SELECT * FROM fulfillment_exception_cases
            WHERE status IN
              <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
            ORDER BY severity, created_at
            LIMIT #{limit}
            </script>
            """)
    List<FulfillmentExceptionCaseEntity> listQueue(@Param("statuses") List<String> statuses,
                                                   @Param("limit") int limit);

    /**
     * 指派承辦人並推進狀態（CAS）。
     *
     * <p>🔴 {@code chk_exc_owner} 要求「離開 OPEN ⇒ 必有 owner ＋ assignedAt」，所以這三件
     * <b>必須同一句寫</b>；拆成兩句會在第一句就被 DB 擋下。
     *
     * @return 1 = 成功；0 = 已不在 allowed-from（<b>不得</b>讀成「別人先做過了」就放行）
     */
    @Update("""
            <script>
            UPDATE fulfillment_exception_cases
            SET status = #{toStatus},
                status_entered_at = NOW(),
                owner_account_id = #{ownerAccountId},
                owner_assigned_at = NOW(),
                updated_at = NOW()
            WHERE id = #{id}
              AND status IN
              <foreach collection="allowedFrom" item="s" open="(" separator="," close=")">#{s}</foreach>
            </script>
            """)
    int assignOwner(@Param("id") Long id,
                    @Param("toStatus") String toStatus,
                    @Param("ownerAccountId") Long ownerAccountId,
                    @Param("allowedFrom") List<String> allowedFrom);

    /**
     * 結案（CAS）。
     *
     * <p>🔴 {@code chk_exc_resolved} 要求結案三件套（{@code resolved_at}／
     * {@code resolved_by_account_id}／{@code resolution_code}）同時有值 ⇒ 同一句寫。
     */
    @Update("""
            <script>
            UPDATE fulfillment_exception_cases
            SET status = #{toStatus},
                status_entered_at = NOW(),
                resolution_code = #{resolutionCode},
                resolution_note = #{resolutionNote,jdbcType=VARCHAR},
                resolved_by_account_id = #{resolvedByAccountId},
                resolved_at = #{resolvedAt},
                updated_at = NOW()
            WHERE id = #{id}
              AND status IN
              <foreach collection="allowedFrom" item="s" open="(" separator="," close=")">#{s}</foreach>
            </script>
            """)
    int resolve(@Param("id") Long id,
                @Param("toStatus") String toStatus,
                @Param("resolutionCode") String resolutionCode,
                @Param("resolutionNote") String resolutionNote,
                @Param("resolvedByAccountId") Long resolvedByAccountId,
                @Param("resolvedAt") LocalDateTime resolvedAt,
                @Param("allowedFrom") List<String> allowedFrom);

    /**
     * Admin 例外案件佇列。
     *
     * <p>🔴 本 entity 有兩個 JSONB 欄位 ⇒ 必須掛 {@code @ResultMap}，否則取回時不走 TypeHandler。
     */
    @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")
    @Select("""
            <script>
            SELECT c.* FROM fulfillment_exception_cases c
            <where>
              <if test="statuses != null and statuses.size() > 0">
                AND c.status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="severity != null">AND c.severity = #{severity}</if>
              <if test="category != null">AND c.category = #{category}</if>
              <if test="subjectType != null">AND c.subject_type = #{subjectType}</if>
              <if test="subjectRef != null">AND c.subject_ref = #{subjectRef}</if>
              <if test="storeId != null">AND c.store_id = #{storeId}</if>
              <if test="awaitingReviewOnly">
                AND EXISTS (SELECT 1 FROM fulfillment_case_resolutions r
                            WHERE r.case_id = c.id AND r.status = #{awaitingReview})
              </if>
            </where>
            ORDER BY c.severity, c.created_at DESC, c.id DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<FulfillmentExceptionCaseEntity> adminListExceptionCases(@Param("statuses") List<String> statuses,
                                                                 @Param("severity") String severity,
                                                                 @Param("category") String category,
                                                                 @Param("subjectType") String subjectType,
                                                                 @Param("subjectRef") String subjectRef,
                                                                 @Param("storeId") Long storeId,
                                                                 @Param("awaitingReviewOnly") boolean awaitingReviewOnly,
                                                                 @Param("awaitingReview") String awaitingReview,
                                                                 @Param("limit") int limit,
                                                                 @Param("offset") int offset);

    /** {@link #adminListExceptionCases} 的總筆數（述詞逐字相同）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM fulfillment_exception_cases c
            <where>
              <if test="statuses != null and statuses.size() > 0">
                AND c.status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="severity != null">AND c.severity = #{severity}</if>
              <if test="category != null">AND c.category = #{category}</if>
              <if test="subjectType != null">AND c.subject_type = #{subjectType}</if>
              <if test="subjectRef != null">AND c.subject_ref = #{subjectRef}</if>
              <if test="storeId != null">AND c.store_id = #{storeId}</if>
              <if test="awaitingReviewOnly">
                AND EXISTS (SELECT 1 FROM fulfillment_case_resolutions r
                            WHERE r.case_id = c.id AND r.status = #{awaitingReview})
              </if>
            </where>
            </script>
            """)
    long adminCountExceptionCases(@Param("statuses") List<String> statuses,
                                  @Param("severity") String severity,
                                  @Param("category") String category,
                                  @Param("subjectType") String subjectType,
                                  @Param("subjectRef") String subjectRef,
                                  @Param("storeId") Long storeId,
                                  @Param("awaitingReviewOnly") boolean awaitingReviewOnly,
                                  @Param("awaitingReview") String awaitingReview);
}
