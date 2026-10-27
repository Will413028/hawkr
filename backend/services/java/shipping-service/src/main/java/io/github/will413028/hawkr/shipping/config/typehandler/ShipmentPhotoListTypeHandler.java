package io.github.will413028.hawkr.shipping.config.typehandler;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity.ShipmentPhoto;
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
 * 出貨照片 JSONB TypeHandler
 * <p>
 * 將 List&lt;ShipmentPhoto&gt; 與 PostgreSQL JSONB 欄位互相轉換。
 * JSONB key 一律 camelCase（裸 ObjectMapper 預設）。
 */
@MappedJdbcTypes(JdbcType.OTHER)
public class ShipmentPhotoListTypeHandler extends BaseTypeHandler<List<ShipmentPhoto>> {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<List<ShipmentPhoto>> TYPE_REF = new TypeReference<>() {};

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, List<ShipmentPhoto> parameter, JdbcType jdbcType) throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize ShipmentPhoto list to JSONB", e);
        }
    }

    @Override
    public List<ShipmentPhoto> getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public List<ShipmentPhoto> getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public List<ShipmentPhoto> getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private List<ShipmentPhoto> parse(String json) throws SQLException {
        if (json == null) return null;
        try {
            return MAPPER.readValue(json, TYPE_REF);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize ShipmentPhoto list from JSONB", e);
        }
    }
}
