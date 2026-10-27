package io.github.will413028.hawkr.shipping.mapper;

import io.github.will413028.hawkr.shipping.dto.ShipmentAnomalyRow;
import io.github.will413028.hawkr.shipping.dto.ShipmentAnomalyTypeCountRow;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

/**
 * §5.21 配送異常 Dashboard 查詢 mapper。
 *
 * <p>核心邏輯：將 shipments 表透過 CASE WHEN 推導 anomaly_type，再 LEFT JOIN ack 表。
 * 異常存在性由 shipments 既有欄位推導，不存獨立狀態（符合 §22 邊界）。
 *
 * <p>Severity 推導（🔴 <b>exhaustive</b>，逐值列舉 ShipmentAnomalyType 全部 11 值，見 {@link #CASE_SEVERITY}）：
 * - HIGH：PAYUNI_LGS_NO_CODE / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST /
 *   UBER_COURIER_UNASSIGNED / UBER_PICKUP_STUCK / UBER_CREATE_UNKNOWN_UNRESOLVED
 * - MEDIUM：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / DELIVERY_OVERDUE
 * - LOW：RETURN_STUCK / UBER_POD_MISSING
 *
 * <p>RETURN_STUCK 在 aftersale_cases 表，**不在本 mapper Dashboard 查詢範圍**，
 * 僅由 FCM 推播即時送達；Dashboard 賣家可從售後管理介面看。
 */
@Mapper
public interface ShipmentAnomalyQueryMapper {

    /**
     * SQL 共用片段：推導 anomaly_type 的 CASE。
     *
     * <p><b>CASE 分支順序按 severity 降序 + 業務階段排序，首匹配優先</b>，
     * 確保一個 shipment 同時匹配多個分支時，Dashboard 顯示最嚴重的 anomaly。
     *
     * <p>順序語意：
     * <ol>
     *   <li><b>HIGH</b>（物流商已明確異常 / 包裹遺失 / 三段碼缺失）— 賣家不能不知道，最先匹配</li>
     *   <li><b>MEDIUM</b>（賣家可自行介入解決的卡死）— 次優先</li>
     * </ol>
     *
     * <p>未來新增 anomaly_type 或調整 severity 對應時，必須維持本順序語意。
     * Severity 對應規則的權威來源見 {@code ShipmentAnomalyRules.severityOf()}。
     *
     * <p><b>SQL 字面值對應 CodeMeta enum（受 @Select compile-time constant 限制
     * 無法用 enum.value() 拼接，須維護時同步）：</b>
     * <ul>
     *   <li>{@code shipments.status}：'33'=ShippingStatus.ABNORMAL、'44'=PACKAGE_LOST、
     *       '32'=WAITING_PICKUP、'11'=PICKED_UP、'41'=CANCELLED</li>
     *   <li>{@code shipping_method}：'3001'=ShippingMethod.TAQBIN、'3002'=SEVEN_ELEVEN_STORE_PICKUP</li>
     *   <li>{@code tcat_call_status}：'4202'=TcatCallStatus.REQUESTED（4 碼）</li>
     *   <li>{@code label_print_status}：'4102'=LabelPrintStatus.REQUESTED、'4103'=SUCCEEDED（4 碼）</li>
     * </ul>
     */
    String CASE_ANOMALY_TYPE = """
            CASE
              -- HIGH severity（最嚴重，優先顯示）
              WHEN s.status='44' THEN 'PACKAGE_LOST'                      -- ShippingStatus.PACKAGE_LOST
              -- 不含 41 (CANCELLED)：賣家主動取消非物流商異常（stage E2E 抓到 false positive）
              WHEN s.status='33' THEN 'PROVIDER_STATUS_ABNORMAL'          -- ShippingStatus.ABNORMAL
              WHEN s.status='32' AND s.updated_at &lt; NOW() - INTERVAL '7 days' THEN 'PACKAGE_LOST'  -- ShippingStatus.WAITING_PICKUP
              -- 限超商（3002）：宅配走 markLabelPrintSucceededHome PDF 路徑，by-design 不寫三段碼，
              -- 未加 shipping_method 限定會把所有宅配 4103 全部誤標為 PAYUNI_LGS_NO_CODE（設計書 06 §5.21.2）
              -- ⚠️ NULLIF 不可省（issue #2789）：三段碼缺欄位時 webhook 寫進 DB 的是**空字串**不是 NULL
              -- （PayUniWebhookEnvelope.payloadField 對缺欄位回 ""，ShipmentMapper
              --  markLabelPrintSucceededByWebhook 的 SET 直接寫該值、無 NULLIF）
              -- ⇒ 裸 `IS NULL` 恆為 false ⇒ 這類異常在賣家 dashboard **一筆都看不到**。
              -- 這條與 ShippingService 的 webhook Print 路徑耦合：那裡的 publishAnomalyDetected
              -- **不是排程、不會重試**，所以推播若也沒送到，賣家就兩條路都沒有訊號。
              WHEN s.shipping_method='3002' AND s.label_print_status='4103'  -- SEVEN_ELEVEN + LabelPrintStatus.SUCCEEDED
                   AND (NULLIF(s.partner_id, '') IS NULL
                        OR NULLIF(s.odno, '') IS NULL
                        OR NULLIF(s.validation_no, '') IS NULL) THEN 'PAYUNI_LGS_NO_CODE'
              -- MEDIUM severity（賣家可介入解決）
              WHEN s.shipping_method='3001' AND s.tcat_call_status='4202'  -- TAQBIN + TcatCallStatus.REQUESTED
                   AND s.tcat_call_requested_at &lt; NOW() - INTERVAL '24 hours' THEN 'TCAT_CALL_FAILED'
              WHEN s.shipping_method='3002' AND s.label_print_status='4102'  -- SEVEN_ELEVEN + LabelPrintStatus.REQUESTED
                   AND s.label_print_requested_at &lt; NOW() - INTERVAL '48 hours' THEN 'CVS_LABEL_UNPRINTED'
              -- DELIVERY_OVERDUE 排除 PICKED_UP/CANCELLED/PACKAGE_LOST 終態，
              -- 並要求物流商超過 3 天無狀態更新（provider_status_time freshness）；
              -- 純粹 ETA 過但軌跡近期仍活躍視為仍在正常移動，不打異常。
              -- provider_status_time IS NULL 視為「物流商從未回報狀態」屬異常範圍。
              WHEN s.expected_delivery_date IS NOT NULL
                   AND s.expected_delivery_date &lt; CURRENT_DATE
                   AND s.status NOT IN ('11','41','44')
                   AND (s.provider_status_time IS NULL
                        OR s.provider_status_time &lt; NOW() - INTERVAL '3 days') THEN 'DELIVERY_OVERDUE'
              ELSE NULL
            END
            """;

    /**
     * HIGH 群：賣家／營運必須<b>立即介入</b>，等到摘要才處理就失去處理價值。
     *
     * <p>🔴 Uber 三值為何是 HIGH（架構裁決 DEC-20260912-184025 第 17 條）：
     * Uber 是即時配送，整段生命週期只有約 1 小時 ⇒ MEDIUM 的「早晚摘要」對它等於沒有通知。
     * <ul>
     *   <li>{@code UBER_COURIER_UNASSIGNED}（30 分未指派）：買家正在等，賣家可取消或改配送方式</li>
     *   <li>{@code UBER_PICKUP_STUCK}（45 分未取件）：貨已打包擺著，賣家要決定是否改派</li>
     *   <li>{@code UBER_CREATE_UNKNOWN_UNRESOLVED}：這張單究竟有沒有建成不明，
     *       賣家在釐清前不應把貨交給任何人</li>
     * </ul>
     */
    String SEVERITY_HIGH_TYPES =
            "'PAYUNI_LGS_NO_CODE', 'PROVIDER_STATUS_ABNORMAL', 'PACKAGE_LOST', "
                    + "'UBER_COURIER_UNASSIGNED', 'UBER_PICKUP_STUCK', 'UBER_CREATE_UNKNOWN_UNRESOLVED'";

    /** MEDIUM 群：賣家可自行介入解決的卡死，早晚摘要即可。 */
    String SEVERITY_MEDIUM_TYPES =
            "'TCAT_CALL_FAILED', 'CVS_LABEL_UNPRINTED', 'DELIVERY_OVERDUE'";

    /**
     * LOW 群：純資訊型，不需人立即介入。
     *
     * <p>{@code UBER_POD_MISSING}（delivered 後 7 天未取得 POD）是純證據面 —— 貨已經送到，
     * 賣家當下無事可做，缺的是事後爭議時的憑證。
     */
    String SEVERITY_LOW_TYPES = "'RETURN_STUCK', 'UBER_POD_MISSING'";

    /**
     * SQL 共用片段：依 a.anomaly_type 推導 severity。
     * <p>SELECT 與 WHERE 子句共用，全部帶 {@code a.} alias 是因為下方所有查詢
     * 都帶 {@code LEFT JOIN shipment_anomaly_acknowledgments ack}，而 ack 表
     * 也有 {@code anomaly_type} 欄位，不帶 alias 時 PostgreSQL 會回
     * {@code column reference "anomaly_type" is ambiguous}（local E2E 抓到的 bug）。
     *
     * <p>🔴 <b>exhaustive：三個群組逐值列舉 {@code ShipmentAnomalyType} 的全部 11 值</b>
     * （架構裁決 DEC-20260912-184025 第 17 條：「不得使用『未知 enum → ELSE LOW』的
     * fail-silent SQL」）。舊版只列 6 值並以 {@code ELSE 'LOW'} 收尾 ⇒ Uber 四值中
     * 判為 HIGH 的三個會在 Dashboard 顯示「低」：賣家收得到即時推播、卻在列表上看到不急，
     * 而這正是「靜默降級」最貴的形狀 —— 它看起來一切正常。
     *
     * <p><b>{@code ELSE 'HIGH'} 不是預設值，是 fail-closed 的最後一道</b>：
     * 三個群組已涵蓋全部 enum 值 ⇒ ELSE 在正確狀態下<b>不可達</b>。它只會在
     * 「有人新增了 ShipmentAnomalyType 卻沒同步本片段」時被走到，而此時
     * <ul>
     *   <li>{@code 'LOW'}：把未知異常降級藏起來（❌ 裁決明令禁止）</li>
     *   <li>{@code NULL}：{@code nullToEmpty} 後前端顯示空白，且 severity 過濾一律不命中
     *       ⇒ 該列從任何有 severity 條件的視圖<b>消失</b>（❌ 同樣是靜默）</li>
     *   <li>{@code 'HIGH'}：留在最高注意力的桶裡，不可能被過濾掉或被忽略（✅ fail-closed）</li>
     * </ul>
     * ⚠️ ELSE 的職責是「不說謊」，<b>不是</b>「偵測」。偵測要靠 CI guard（見下）。
     *
     * <p><b>⚠ Java helper 同步點</b>：本對應規則的權威來源在
     * {@code ShipmentAnomalyRules.severityOf()}（common-core），該 helper 用於 FCM 推送 /
     * inbox 寫入時推導 severity。本 SQL 為 Dashboard 端的對應實作 —— 兩者是
     * <b>雙份 SSOT</b>，修改任一邊都必須同步另一邊，否則 Dashboard 顯示的 severity
     * 與推送 severity 會脫節。
     * <p>🔴 <b>為什麼沒有整併掉這份重複</b>（誠實標註，2026-09-12）：
     * {@code @Select} 的值必須是<b>編譯期常數</b>，無法呼叫 {@code ShipmentAnomalyType.values()}
     * 或 {@code severityOf()} 生成；真要單一 SSOT 只能把 6 個 {@code @Select} 全數改成
     * {@code @SelectProvider}，而 shipping-service <b>沒有 DB 測試基礎設施</b>
     * （testcontainers 只在 cart / aftersales / coupon，且被預設 {@code mvn test} 排除）
     * ⇒ 改寫後的動態 SQL 無法被驗證。在賣家 Dashboard 這條線上做無法驗證的改寫，
     * 代價大於它消除的重複 ⇒ 本波改以「exhaustive 列舉 ＋ fail-closed ELSE ＋ CI guard」處理。
     * <p>本片段被刻意拆成三個具名常數（{@link #SEVERITY_HIGH_TYPES} /
     * {@link #SEVERITY_MEDIUM_TYPES} / {@link #SEVERITY_LOW_TYPES}）就是為了讓 guard
     * 有<b>結構上的錨</b>可解析，而不是去正則比對一坨埋在 CASE 裡的字串。
     *
     * <p>⚠️ <b>本片段目前對 Uber 四值不可觀測</b>（不要把它當成已驗證的線上行為）：
     * {@link #CASE_ANOMALY_TYPE} 由 {@code shipments} 既有欄位推導，<b>不產出任何 UBER_* 值</b>
     * ⇒ 本次修正是 66 W2 接上 emitter 前的<b>前置</b>。W2 擴充 CASE_ANOMALY_TYPE 時，
     * 本片段已經就位，不需要再記得回來改一次。
     */
    String CASE_SEVERITY =
            "CASE\n"
                    + "  WHEN a.anomaly_type IN (" + SEVERITY_HIGH_TYPES + ") THEN 'HIGH'\n"
                    + "  WHEN a.anomaly_type IN (" + SEVERITY_MEDIUM_TYPES + ") THEN 'MEDIUM'\n"
                    + "  WHEN a.anomaly_type IN (" + SEVERITY_LOW_TYPES + ") THEN 'LOW'\n"
                    // fail-closed：不可達；可達即代表 enum 新增後沒同步本片段。
                    + "  ELSE 'HIGH'\n"
                    + "END\n";

    /**
     * 賣家 Dashboard 列表查詢。
     */
    @Select({"<script>",
            "WITH abnormal_shipments AS (",
            "  SELECT s.id, s.order_no, s.store_id, s.status AS shipment_status,",
            "         s.shipping_method, s.updated_at,", CASE_ANOMALY_TYPE, "AS anomaly_type",
            "  FROM shipments s",
            "  WHERE s.store_id = #{sellerId}",
            "    AND (s.is_return = FALSE OR s.is_return IS NULL)",
            ")",
            "SELECT a.id AS shipment_id, a.order_no, a.shipment_status, a.shipping_method,",
            "       a.anomaly_type,", CASE_SEVERITY, "AS severity,",
            "       a.updated_at AS detected_at, ack.action AS ack_action, ack.acted_at",
            "FROM abnormal_shipments a",
            "LEFT JOIN shipment_anomaly_acknowledgments ack",
            "       ON ack.shipment_id = a.id AND ack.anomaly_type = a.anomaly_type",
            // issue #1327：賣家 dashboard 只看賣家自己的 ack，看不到 admin 標記
            "      AND ack.actor_type = 'SELLER'",
            "WHERE a.anomaly_type IS NOT NULL",
            "  <if test='anomalyType != null and anomalyType != \"\"'>",
            "    AND a.anomaly_type = #{anomalyType}",
            "  </if>",
            "  <if test='severity != null and severity != \"\"'>",
            "    AND (", CASE_SEVERITY, ") = #{severity}",
            "  </if>",
            "  <if test='!showAcknowledged'>",
            "    AND ack.id IS NULL",
            "  </if>",
            "ORDER BY a.updated_at DESC",
            "LIMIT #{limit} OFFSET #{offset}",
            "</script>"})
    List<ShipmentAnomalyRow> selectSellerAnomalies(@Param("sellerId") long sellerId,
                                                   @Param("anomalyType") String anomalyType,
                                                   @Param("severity") String severity,
                                                   @Param("showAcknowledged") boolean showAcknowledged,
                                                   @Param("limit") int limit,
                                                   @Param("offset") int offset);

    /**
     * 賣家 Dashboard 列表 total。
     */
    @Select({"<script>",
            "WITH abnormal_shipments AS (",
            "  SELECT s.id,", CASE_ANOMALY_TYPE, "AS anomaly_type",
            "  FROM shipments s",
            "  WHERE s.store_id = #{sellerId}",
            "    AND (s.is_return = FALSE OR s.is_return IS NULL)",
            ")",
            "SELECT COUNT(*)",
            "FROM abnormal_shipments a",
            "LEFT JOIN shipment_anomaly_acknowledgments ack",
            "       ON ack.shipment_id = a.id AND ack.anomaly_type = a.anomaly_type",
            "      AND ack.actor_type = 'SELLER'",  // issue #1327：賣家端只計自己的 ack
            "WHERE a.anomaly_type IS NOT NULL",
            "  <if test='anomalyType != null and anomalyType != \"\"'>",
            "    AND a.anomaly_type = #{anomalyType}",
            "  </if>",
            "  <if test='severity != null and severity != \"\"'>",
            "    AND (", CASE_SEVERITY, ") = #{severity}",
            "  </if>",
            "  <if test='!showAcknowledged'>",
            "    AND ack.id IS NULL",
            "  </if>",
            "</script>"})
    long countSellerAnomalies(@Param("sellerId") long sellerId,
                              @Param("anomalyType") String anomalyType,
                              @Param("severity") String severity,
                              @Param("showAcknowledged") boolean showAcknowledged);

    /**
     * 賣家 Dashboard 統計：依 anomaly_type 分組（未 ack 部分）。
     */
    @Select({"<script>",
            "WITH abnormal_shipments AS (",
            "  SELECT s.id,", CASE_ANOMALY_TYPE, "AS anomaly_type",
            "  FROM shipments s",
            "  WHERE s.store_id = #{sellerId}",
            "    AND (s.is_return = FALSE OR s.is_return IS NULL)",
            ")",
            "SELECT a.anomaly_type, COUNT(*) AS cnt",
            "FROM abnormal_shipments a",
            "LEFT JOIN shipment_anomaly_acknowledgments ack",
            "       ON ack.shipment_id = a.id AND ack.anomaly_type = a.anomaly_type",
            "      AND ack.actor_type = 'SELLER'",  // issue #1327：賣家統計只看自己的 ack
            "WHERE a.anomaly_type IS NOT NULL",
            "  AND ack.id IS NULL",
            "GROUP BY a.anomaly_type",
            "</script>"})
    List<ShipmentAnomalyTypeCountRow> countSellerAnomaliesByType(@Param("sellerId") long sellerId);

    /**
     * Admin 端列表查詢（可選 sellerId 過濾）。
     */
    @Select({"<script>",
            "WITH abnormal_shipments AS (",
            "  SELECT s.id, s.order_no, s.store_id, s.status AS shipment_status,",
            "         s.shipping_method, s.updated_at,", CASE_ANOMALY_TYPE, "AS anomaly_type",
            "  FROM shipments s",
            "  WHERE (s.is_return = FALSE OR s.is_return IS NULL)",
            "    <if test='sellerId != null and sellerId > 0'>",
            "      AND s.store_id = #{sellerId}",
            "    </if>",
            ")",
            // issue #1327：ack 拆兩路 LEFT JOIN 揭露雙 actor 處置狀態。
            // ack_action/acted_at = admin 端處置（aa）；seller_ack_action/seller_acted_at = 賣家端處置（sa，唯讀揭露）
            "SELECT a.id AS shipment_id, a.order_no, a.store_id, a.shipment_status,",
            "       a.shipping_method, a.anomaly_type,", CASE_SEVERITY, "AS severity,",
            "       a.updated_at AS detected_at,",
            "       aa.action AS ack_action, aa.acted_at AS acted_at,",
            "       sa.action AS seller_ack_action, sa.acted_at AS seller_acted_at",
            "FROM abnormal_shipments a",
            "LEFT JOIN shipment_anomaly_acknowledgments sa",
            "       ON sa.shipment_id = a.id AND sa.anomaly_type = a.anomaly_type AND sa.actor_type = 'SELLER'",
            "LEFT JOIN shipment_anomaly_acknowledgments aa",
            "       ON aa.shipment_id = a.id AND aa.anomaly_type = a.anomaly_type AND aa.actor_type = 'ADMIN'",
            "WHERE a.anomaly_type IS NOT NULL",
            "  <if test='anomalyType != null and anomalyType != \"\"'>",
            "    AND a.anomaly_type = #{anomalyType}",
            "  </if>",
            "  <if test='severity != null and severity != \"\"'>",
            "    AND (", CASE_SEVERITY, ") = #{severity}",
            "  </if>",
            "  <if test='!showAcknowledged'>",
            "    AND aa.id IS NULL",  // 預設隱藏 admin 已處置者
            "  </if>",
            "ORDER BY a.updated_at DESC",
            "LIMIT #{limit} OFFSET #{offset}",
            "</script>"})
    List<ShipmentAnomalyRow> selectAdminAnomalies(@Param("sellerId") Long sellerId,
                                                  @Param("anomalyType") String anomalyType,
                                                  @Param("severity") String severity,
                                                  @Param("showAcknowledged") boolean showAcknowledged,
                                                  @Param("limit") int limit,
                                                  @Param("offset") int offset);

    @Select({"<script>",
            "WITH abnormal_shipments AS (",
            "  SELECT s.id,", CASE_ANOMALY_TYPE, "AS anomaly_type",
            "  FROM shipments s",
            "  WHERE (s.is_return = FALSE OR s.is_return IS NULL)",
            "    <if test='sellerId != null and sellerId > 0'>",
            "      AND s.store_id = #{sellerId}",
            "    </if>",
            ")",
            "SELECT COUNT(*)",
            "FROM abnormal_shipments a",
            "LEFT JOIN shipment_anomaly_acknowledgments aa",
            "       ON aa.shipment_id = a.id AND aa.anomaly_type = a.anomaly_type AND aa.actor_type = 'ADMIN'",
            "WHERE a.anomaly_type IS NOT NULL",
            "  <if test='anomalyType != null and anomalyType != \"\"'>",
            "    AND a.anomaly_type = #{anomalyType}",
            "  </if>",
            "  <if test='severity != null and severity != \"\"'>",
            "    AND (", CASE_SEVERITY, ") = #{severity}",
            "  </if>",
            "  <if test='!showAcknowledged'>",
            "    AND aa.id IS NULL",  // 預設隱藏 admin 已處置者
            "  </if>",
            "</script>"})
    long countAdminAnomalies(@Param("sellerId") Long sellerId,
                             @Param("anomalyType") String anomalyType,
                             @Param("severity") String severity,
                             @Param("showAcknowledged") boolean showAcknowledged);
}
