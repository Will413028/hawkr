package io.github.will413028.hawkr.product.config.typehandler;

import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity.GovVerifyPayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.MappedJdbcTypes;
import org.apache.ibatis.type.MappedTypes;
import org.postgresql.util.PGobject;

import java.sql.CallableStatement;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;

/**
 * {@code product_certification_application.gov_verify_payload} JSONB TypeHandler（19 §4.1／§8.3）。
 *
 * <p>依 Java 微服務開發規範：每個 JSONB 結構一個 TypeHandler、key 一律 camelCase、
 * 使用裸 {@code new ObjectMapper()}（不套 SNAKE_CASE 命名策略），
 * 反序列化忽略未知欄位以便政府查驗結果 additive 擴充時舊列仍讀得回來。
 */
@MappedTypes(GovVerifyPayload.class)
@MappedJdbcTypes(JdbcType.OTHER)
public class GovVerifyPayloadTypeHandler extends BaseTypeHandler<GovVerifyPayload> {

    private static final ObjectMapper MAPPER = new ObjectMapper()
            .configure(com.fasterxml.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, GovVerifyPayload parameter, JdbcType jdbcType)
            throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize GovVerifyPayload to JSONB", e);
        }
    }

    @Override
    public GovVerifyPayload getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public GovVerifyPayload getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public GovVerifyPayload getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private GovVerifyPayload parse(String json) throws SQLException {
        if (json == null) {
            return null;
        }
        try {
            return MAPPER.readValue(json, GovVerifyPayload.class);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize GovVerifyPayload from JSONB", e);
        }
    }
}
