package io.github.will413028.hawkr.common.mybatis.typehandler;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.MappedJdbcTypes;

import java.sql.CallableStatement;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Types;
import java.util.Map;

/**
 * JSONB Map&lt;String,Object&gt; TypeHandler（opaque passthrough，跨服務共用）。
 * <p>
 * 以裸 ObjectMapper（不套命名策略）原樣序列化，key 不做任何轉換——用於「忠實稽核第三方原文」類欄位
 * （如 payuni_audit_logs 的 request_params / response_decrypted，保留 PayUni 原生 PascalCase key，
 * 見 05 §11.2.1 的 §11.1 camelCase 白名單例外）。
 * <p>
 * 寫入走 {@code setObject(i, json, Types.OTHER)}（PostgreSQL JDBC 標準 jsonb 綁定），
 * 不依賴 postgresql driver 類別，common-core 無需新增依賴。
 */
@MappedJdbcTypes(JdbcType.OTHER)
public class JsonbMapTypeHandler extends BaseTypeHandler<Map<String, Object>> {

    private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, Map<String, Object> parameter, JdbcType jdbcType)
            throws SQLException {
        try {
            ps.setObject(i, OBJECT_MAPPER.writeValueAsString(parameter), Types.OTHER);
        } catch (JsonProcessingException e) {
            throw new SQLException("轉換 Map 到 JSON 失敗", e);
        }
    }

    @Override
    public Map<String, Object> getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parseJson(rs.getString(columnName));
    }

    @Override
    public Map<String, Object> getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parseJson(rs.getString(columnIndex));
    }

    @Override
    public Map<String, Object> getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parseJson(cs.getString(columnIndex));
    }

    private Map<String, Object> parseJson(String json) throws SQLException {
        if (json == null || json.isEmpty()) {
            return null;
        }
        try {
            return OBJECT_MAPPER.readValue(json, new TypeReference<>() {});
        } catch (JsonProcessingException e) {
            throw new SQLException("解析 JSON 到 Map 失敗", e);
        }
    }
}
