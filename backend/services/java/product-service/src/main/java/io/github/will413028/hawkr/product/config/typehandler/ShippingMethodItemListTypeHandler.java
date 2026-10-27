package io.github.will413028.hawkr.product.config.typehandler;

import io.github.will413028.hawkr.product.entity.ProductSkuEntity.ShippingMethodItem;
import com.fasterxml.jackson.core.type.TypeReference;
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
import java.util.List;

@MappedTypes(List.class)
@MappedJdbcTypes(JdbcType.OTHER)
public class ShippingMethodItemListTypeHandler extends BaseTypeHandler<List<ShippingMethodItem>> {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<List<ShippingMethodItem>> TYPE_REF = new TypeReference<>() {};

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, List<ShippingMethodItem> parameter, JdbcType jdbcType) throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize ShippingMethodItem list to JSONB", e);
        }
    }

    @Override
    public List<ShippingMethodItem> getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public List<ShippingMethodItem> getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public List<ShippingMethodItem> getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private List<ShippingMethodItem> parse(String json) throws SQLException {
        if (json == null) return null;
        try {
            return MAPPER.readValue(json, TYPE_REF);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize ShippingMethodItem list from JSONB", e);
        }
    }
}
