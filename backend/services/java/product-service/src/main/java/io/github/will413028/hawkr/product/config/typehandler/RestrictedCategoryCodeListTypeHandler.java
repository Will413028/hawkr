package io.github.will413028.hawkr.product.config.typehandler;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.MappedJdbcTypes;
import org.postgresql.util.PGobject;

import java.sql.CallableStatement;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.List;

/**
 * {@code product_sku.restricted_category_codes} 的 JSONB TypeHandler
 * （66 號詳細設計書 §5.1.9 ②：禁運品／品類守門的 SKU 標記欄）。
 *
 * <p>內容是 {@code RestrictedGoodsCategory} 的 Value 字串陣列，例如 {@code ["...", "..."]}；
 * 空陣列 = 未標記任何受限品類。
 *
 * <p>🔴 <b>值域待 UP-4 定案，在那之前 DB 的
 * {@code chk_product_sku_restricted_category_codes} 只允許空陣列</b>
 * （V19 migration；「沒有碼表」必須等於「寫不進去」，不是「什麼都寫得進去」）。
 * 本 handler 刻意<b>不</b>做值域驗證：值域的 SSOT 是 CodeMeta，而它現在是空的，
 * 在這裡複製一份空清單只會變成第二個要維護的地方。UP-4 定案後由寫入路徑走
 * {@code CodeMetaValidator.validate(RestrictedGoodsCategory.class, ...)}，
 * DB CHECK 則是繞過應用層時的最後一道。
 *
 * <p>⚠️ 刻意<b>不</b>標 {@code @MappedTypes(List.class)}
 * （《Java 微服務開發規範》§9.1 明令禁止）：本服務已有兩個 {@code List} 的 JSONB handler，
 * 再宣告一次只會讓 {@code TypeHandlerRegistry} 裡 {@code List + OTHER} 這一格再被覆寫一次。
 * 本 handler 一律由 {@code @TableField(typeHandler = ...)} 顯式指名使用，不依賴隱式查表。
 *
 * <p>ObjectMapper 用裸 {@code new ObjectMapper()}（規範 §9.1）——預設即 camelCase，
 * 不得設 {@code PropertyNamingStrategies.SNAKE_CASE}。本欄是純字串陣列、沒有物件 key，
 * 但仍沿用同一條規則，避免下一個人以為這裡可以自由設定。
 */
@MappedJdbcTypes(JdbcType.OTHER)
public class RestrictedCategoryCodeListTypeHandler extends BaseTypeHandler<List<String>> {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<List<String>> TYPE_REF = new TypeReference<>() {};

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, List<String> parameter, JdbcType jdbcType) throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize restricted category code list to JSONB", e);
        }
    }

    @Override
    public List<String> getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public List<String> getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public List<String> getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private List<String> parse(String json) throws SQLException {
        if (json == null) return null;
        try {
            return MAPPER.readValue(json, TYPE_REF);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize restricted category code list from JSONB", e);
        }
    }
}
