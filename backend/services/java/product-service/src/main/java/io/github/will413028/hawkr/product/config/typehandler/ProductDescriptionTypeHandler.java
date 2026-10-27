package io.github.will413028.hawkr.product.config.typehandler;

import io.github.will413028.hawkr.product.entity.ProductEntity.ProductDescription;
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

@MappedTypes(ProductDescription.class)
@MappedJdbcTypes(JdbcType.OTHER)
public class ProductDescriptionTypeHandler extends BaseTypeHandler<ProductDescription> {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, ProductDescription parameter, JdbcType jdbcType) throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize ProductDescription to JSONB", e);
        }
    }

    @Override
    public ProductDescription getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public ProductDescription getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public ProductDescription getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private ProductDescription parse(String json) throws SQLException {
        if (json == null) return null;
        try {
            return MAPPER.readValue(json, ProductDescription.class);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize ProductDescription from JSONB", e);
        }
    }
}
