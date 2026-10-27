package io.github.will413028.hawkr.shipping.mapper;

import io.github.will413028.hawkr.shipping.dto.UberDeliveryRow;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Uber 配送查詢 mapper（66；Stage 8B W6-3 賣家面／W6-4 Admin 面）。
 *
 * <h2>🔴 驅動表是 {@code shipments} 不是 {@code uber_deliveries}</h2>
 * 建單失敗（{@code 990 CREATE_FAILED}）與建單結果未知（{@code 991 CREATE_UNKNOWN}）的單
 * <b>根本沒有 {@code uber_deliveries} 列</b>（66 §5.1.5：建單成功才寫入）。而 W6-4 明列
 * 要能查「failure」與「unknown」——以 {@code uber_deliveries} 當驅動表，最需要被看見的
 * 那幾筆會一筆都不在列表裡，而且完全無聲（查詢照樣回 200、照樣有資料）。
 *
 * <h2>🔴 歸屬（ownership）只在一個地方</h2>
 * 賣家面的三支查詢（list／count／detail）全部帶 {@link #SELLER_OWNERSHIP_PREDICATE}。
 * {@code uber_deliveries} 自己<b>沒有 store_id</b>，歸屬的唯一來源是
 * {@code shipments.store_id} ⇒ 把歸屬判定放在 Java 端就等於多一條可以繞過的路徑。
 * 這三處由 {@code UberDeliveryQueryMapperSqlTest} 逐支釘住（三支都要釘：
 * 只釘 list 的話，把 detail 的述詞拿掉一樣是全綠）。
 *
 * <h2>⚠️ 本 mapper 的 SQL 字面值對應 CodeMeta enum</h2>
 * {@code @Select} 的值必須是編譯期常數，無法呼叫 {@code ShippingMethod.UBER_INSTANT.value()}
 * 拼接（同 {@link ShipmentAnomalyQueryMapper} 的既有處置）：
 * <ul>
 *   <li>{@code '3003'} ＝ {@code ShippingMethod.UBER_INSTANT}</li>
 *   <li>{@code '991'} ＝ {@code ShippingStatus.CREATE_UNKNOWN}</li>
 * </ul>
 * 兩者都由 {@code UberDeliveryQueryMapperSqlTest} 對 enum 實值反查釘住，避免 enum 改值後
 * 這裡靜默指向不存在的碼。
 */
@Mapper
public interface UberDeliveryQueryMapper {

    /**
     * 🔴 賣家歸屬述詞——<b>本檔唯一的 ownership 判定</b>。
     *
     * <p>拿掉它的後果不是 403 而是「賣家看到全平台的 Uber 配送」，包含別人的訂單編號、
     * 收件狀態與外送員姓名；而列表照樣回 200、照樣分頁正常 ⇒ 沒有任何訊號。
     */
    String SELLER_OWNERSHIP_PREDICATE = "AND s.store_id = #{storeId}";

    /** 只看 Uber 即時配送（{@code ShippingMethod.UBER_INSTANT}）。 */
    String UBER_METHOD_PREDICATE = "AND s.shipping_method = '3003'";

    /**
     * 摘要欄位。
     *
     * <p>{@code delivery_no} 是<b>可定址鍵</b>：有 {@code uber_deliveries} 列時取 provider 的
     * {@code delivery_id}，沒有時退回我方 {@code shipment_no}。兩個值域不相交
     * （{@code del_}／{@code ret_} vs {@code SH} 前綴）⇒ 單一路徑參數就能打開<b>每一列</b>，
     * 包含建單失敗那些沒有 provider id 的。
     *
     * <p>JSONB 兩欄顯式 {@code CAST(... AS TEXT)}：不轉的話 JDBC 回 {@code PGobject}，
     * 而本行是 POJO 沒有掛 TypeHandler。
     */
    String SELECT_COLUMNS = """
            s.id AS shipment_id, s.shipment_no, s.order_no, s.store_id, s.buyer_id,
            s.shipping_method, s.is_return, s.status,
            s.provider_error_code, s.provider_error_message, s.provider_error_at,
            s.created_at, s.updated_at,
            d.delivery_id, d.quote_id, d.provider_status, d.courier_imminent,
            d.courier_name, d.courier_phone_masked,
            CAST(d.courier_location AS TEXT) AS courier_location,
            CAST(d.related_deliveries AS TEXT) AS related_deliveries,
            d.eta_dropoff_at, d.undeliverable_reason, d.cancelation_reason, d.outcome,
            COALESCE(NULLIF(d.delivery_id, ''), s.shipment_no) AS delivery_no,
            (SELECT COUNT(*) FROM uber_pod_artifacts p
              WHERE p.delivery_id = d.delivery_id AND p.fetched_at IS NOT NULL) AS pod_artifact_count
            """;

    /** 1:1（{@code uk_uber_deliveries_shipment_id}）⇒ LEFT JOIN 不會放大列數。 */
    String FROM_JOIN = "FROM shipments s LEFT JOIN uber_deliveries d ON d.shipment_id = s.id";

    /** 賣家／Admin 共用的可選篩選（不含歸屬與 Admin 專屬項）。 */
    String COMMON_FILTERS = """
            <if test='statusList != null and statusList.size() > 0'>
              AND s.status IN
              <foreach item='st' collection='statusList' open='(' separator=',' close=')'>#{st}</foreach>
            </if>
            <if test='providerStatus != null and providerStatus != ""'>
              AND d.provider_status = #{providerStatus}
            </if>
            <if test='orderNo != null and orderNo != ""'>
              AND s.order_no = #{orderNo}
            </if>
            <if test='outcome != null and outcome != ""'>
              AND d.outcome = #{outcome}
            </if>
            <if test='createdFrom != null'>
              AND s.created_at &gt;= #{createdFrom}
            </if>
            <if test='createdTo != null'>
              AND s.created_at &lt; #{createdTo}
            </if>
            """;

    /** 排序白名單。{@code sort} 由 service 正規化過，這裡只做字面對應（ORDER BY 無法參數化）。 */
    String ORDER_BY = """
            <choose>
              <when test='sort == "updated_at_asc"'>ORDER BY s.updated_at ASC, s.id ASC</when>
              <when test='sort == "created_at_desc"'>ORDER BY s.created_at DESC, s.id DESC</when>
              <when test='sort == "created_at_asc"'>ORDER BY s.created_at ASC, s.id ASC</when>
              <otherwise>ORDER BY s.updated_at DESC, s.id DESC</otherwise>
            </choose>
            """;

    // ==================== 賣家面 ====================

    @Select({"<script>",
            "SELECT", SELECT_COLUMNS,
            FROM_JOIN,
            "WHERE 1=1",
            SELLER_OWNERSHIP_PREDICATE,
            UBER_METHOD_PREDICATE,
            "<if test='isReturn != null'>AND s.is_return = #{isReturn}</if>",
            COMMON_FILTERS,
            ORDER_BY,
            "LIMIT #{limit} OFFSET #{offset}",
            "</script>"})
    List<UberDeliveryRow> selectSellerDeliveries(@Param("storeId") long storeId,
                                                 @Param("statusList") List<String> statusList,
                                                 @Param("providerStatus") String providerStatus,
                                                 @Param("orderNo") String orderNo,
                                                 @Param("isReturn") Boolean isReturn,
                                                 @Param("outcome") String outcome,
                                                 @Param("createdFrom") LocalDateTime createdFrom,
                                                 @Param("createdTo") LocalDateTime createdTo,
                                                 @Param("sort") String sort,
                                                 @Param("limit") int limit,
                                                 @Param("offset") int offset);

    @Select({"<script>",
            "SELECT COUNT(*)",
            FROM_JOIN,
            "WHERE 1=1",
            SELLER_OWNERSHIP_PREDICATE,
            UBER_METHOD_PREDICATE,
            "<if test='isReturn != null'>AND s.is_return = #{isReturn}</if>",
            COMMON_FILTERS,
            "</script>"})
    long countSellerDeliveries(@Param("storeId") long storeId,
                               @Param("statusList") List<String> statusList,
                               @Param("providerStatus") String providerStatus,
                               @Param("orderNo") String orderNo,
                               @Param("isReturn") Boolean isReturn,
                               @Param("outcome") String outcome,
                               @Param("createdFrom") LocalDateTime createdFrom,
                               @Param("createdTo") LocalDateTime createdTo);

    /**
     * 賣家看單筆。
     *
     * <p>🔴 歸屬述詞在<b>同一句 SQL</b> 內：查不到與不屬於你回的是同一件事（空結果），
     * 呼叫端因此無法把「存在但不是你的」與「不存在」區分開來——那正是我們要的
     * （能區分就等於一個可列舉存在性的 oracle）。
     */
    @Select({"<script>",
            "SELECT", SELECT_COLUMNS,
            FROM_JOIN,
            "WHERE 1=1",
            SELLER_OWNERSHIP_PREDICATE,
            UBER_METHOD_PREDICATE,
            "AND (d.delivery_id = #{deliveryNo} OR s.shipment_no = #{deliveryNo})",
            "LIMIT 1",
            "</script>"})
    UberDeliveryRow selectSellerDeliveryDetail(@Param("storeId") long storeId,
                                               @Param("deliveryNo") String deliveryNo);

    // ==================== Admin 面 ====================

    /**
     * Admin 列表。九個篩選維度逐項對應 W6-4「Uber Delivery List」。
     *
     * <p>🔴 {@code failureOnly} 的判準是 {@code outcome} 非空 <b>OR</b> {@code provider_error_code}
     * 非空，<b>不是</b> {@code provider_status} 有值——那欄保存的是正常軌跡
     * （同 V12 對 {@code shipments} 的修正：以「有值」判異常會把每張正常單撈進來）。
     *
     * <p>🔴 {@code unknownOnly}（{@code 991 CREATE_UNKNOWN}）<b>不是</b> failure 的子集：
     * 991 的語意是「可能已經建成功了」，處置是回查而不是重建（66 陷阱 A8：重建＝兩趟車重複計費）。
     * 兩個旗標因此各自獨立，{@code AND} 起來是「未知且已判失敗」＝空集合，那是呼叫端的選擇。
     */
    @Select({"<script>",
            "SELECT", SELECT_COLUMNS,
            FROM_JOIN,
            "WHERE 1=1",
            UBER_METHOD_PREDICATE,
            "<if test='storeId != null'>AND s.store_id = #{storeId}</if>",
            "<if test='shipmentNo != null and shipmentNo != \"\"'>AND s.shipment_no = #{shipmentNo}</if>",
            "<if test='deliveryId != null and deliveryId != \"\"'>",
            //  provider id 兩處都可能有值：uber_deliveries 是主體，shipments.uber_delivery_id 是
            //  月結對帳用的鏡射。只比對其中一邊，會在另一邊先寫入時查不到（而那正是出事時的狀態）。
            "  AND (d.delivery_id = #{deliveryId} OR s.uber_delivery_id = #{deliveryId})",
            "</if>",
            "<if test='failureOnly'>",
            "  AND (d.outcome IS NOT NULL OR NULLIF(s.provider_error_code, '') IS NOT NULL)",
            "</if>",
            //  '991' = ShippingStatus.CREATE_UNKNOWN
            "<if test='unknownOnly'>AND s.status = '991'</if>",
            COMMON_FILTERS,
            ORDER_BY,
            "LIMIT #{limit} OFFSET #{offset}",
            "</script>"})
    List<UberDeliveryRow> selectAdminDeliveries(@Param("storeId") Long storeId,
                                                @Param("shipmentNo") String shipmentNo,
                                                @Param("deliveryId") String deliveryId,
                                                @Param("statusList") List<String> statusList,
                                                @Param("providerStatus") String providerStatus,
                                                @Param("orderNo") String orderNo,
                                                @Param("outcome") String outcome,
                                                @Param("failureOnly") boolean failureOnly,
                                                @Param("unknownOnly") boolean unknownOnly,
                                                @Param("createdFrom") LocalDateTime createdFrom,
                                                @Param("createdTo") LocalDateTime createdTo,
                                                @Param("sort") String sort,
                                                @Param("limit") int limit,
                                                @Param("offset") int offset);

    @Select({"<script>",
            "SELECT COUNT(*)",
            FROM_JOIN,
            "WHERE 1=1",
            UBER_METHOD_PREDICATE,
            "<if test='storeId != null'>AND s.store_id = #{storeId}</if>",
            "<if test='shipmentNo != null and shipmentNo != \"\"'>AND s.shipment_no = #{shipmentNo}</if>",
            "<if test='deliveryId != null and deliveryId != \"\"'>",
            "  AND (d.delivery_id = #{deliveryId} OR s.uber_delivery_id = #{deliveryId})",
            "</if>",
            "<if test='failureOnly'>",
            "  AND (d.outcome IS NOT NULL OR NULLIF(s.provider_error_code, '') IS NOT NULL)",
            "</if>",
            "<if test='unknownOnly'>AND s.status = '991'</if>",
            COMMON_FILTERS,
            "</script>"})
    long countAdminDeliveries(@Param("storeId") Long storeId,
                              @Param("shipmentNo") String shipmentNo,
                              @Param("deliveryId") String deliveryId,
                              @Param("statusList") List<String> statusList,
                              @Param("providerStatus") String providerStatus,
                              @Param("orderNo") String orderNo,
                              @Param("outcome") String outcome,
                              @Param("failureOnly") boolean failureOnly,
                              @Param("unknownOnly") boolean unknownOnly,
                              @Param("createdFrom") LocalDateTime createdFrom,
                              @Param("createdTo") LocalDateTime createdTo);

    @Select({"<script>",
            "SELECT", SELECT_COLUMNS,
            FROM_JOIN,
            "WHERE 1=1",
            UBER_METHOD_PREDICATE,
            "AND (d.delivery_id = #{deliveryNo} OR s.shipment_no = #{deliveryNo})",
            "LIMIT 1",
            "</script>"})
    UberDeliveryRow selectAdminDeliveryDetail(@Param("deliveryNo") String deliveryNo);
}
