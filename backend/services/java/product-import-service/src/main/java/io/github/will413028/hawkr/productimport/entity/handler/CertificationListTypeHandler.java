package io.github.will413028.hawkr.productimport.entity.handler;

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
import java.util.Collections;
import java.util.List;
import java.util.Map;

/**
 * JSONB TypeHandler — product_import_item.certifications。
 *
 * <p>結構：{@code [{name: CertificationType value, certNo: string}]}。
 */
@MappedJdbcTypes(JdbcType.OTHER)
public class CertificationListTypeHandler extends BaseTypeHandler<List<Map<String, Object>>> {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<List<Map<String, Object>>> TYPE_REF = new TypeReference<>() {};

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i,
                                    List<Map<String, Object>> parameter, JdbcType jdbcType) throws SQLException {
        PGobject pg = new PGobject();
        pg.setType("jsonb");
        try {
            pg.setValue(MAPPER.writeValueAsString(parameter));
        } catch (Exception e) {
            throw new SQLException("certifications JSONB serialize failed", e);
        }
        ps.setObject(i, pg);
    }

    @Override
    public List<Map<String, Object>> getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public List<Map<String, Object>> getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public List<Map<String, Object>> getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private List<Map<String, Object>> parse(String raw) throws SQLException {
        if (raw == null || raw.isBlank()) {
            return Collections.emptyList();
        }
        try {
            return MAPPER.readValue(raw, TYPE_REF);
        } catch (Exception e) {
            throw new SQLException("certifications JSONB parse failed", e);
        }
    }
}
