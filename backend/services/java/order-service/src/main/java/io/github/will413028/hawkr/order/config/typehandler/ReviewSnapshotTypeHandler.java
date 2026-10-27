package io.github.will413028.hawkr.order.config.typehandler;

import io.github.will413028.hawkr.order.entity.ReviewSnapshot;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.postgresql.util.PGobject;

import java.sql.CallableStatement;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;

/**
 * ReviewSnapshot ↔ PostgreSQL JSONB TypeHandler（orders.review）。
 *
 * 裸 ObjectMapper（預設 camelCase，§11.1）。NULL JSONB 回傳 null（表未評論）。
 */
public class ReviewSnapshotTypeHandler extends BaseTypeHandler<ReviewSnapshot> {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, ReviewSnapshot parameter, JdbcType jdbcType) throws SQLException {
        try {
            PGobject pgObject = new PGobject();
            pgObject.setType("jsonb");
            pgObject.setValue(MAPPER.writeValueAsString(parameter));
            ps.setObject(i, pgObject);
        } catch (Exception e) {
            throw new SQLException("Failed to serialize ReviewSnapshot to JSONB", e);
        }
    }

    @Override
    public ReviewSnapshot getNullableResult(ResultSet rs, String columnName) throws SQLException {
        return parse(rs.getString(columnName));
    }

    @Override
    public ReviewSnapshot getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        return parse(rs.getString(columnIndex));
    }

    @Override
    public ReviewSnapshot getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        return parse(cs.getString(columnIndex));
    }

    private ReviewSnapshot parse(String json) throws SQLException {
        if (json == null || json.isBlank()) {
            return null;
        }
        try {
            return MAPPER.readValue(json, ReviewSnapshot.class);
        } catch (Exception e) {
            throw new SQLException("Failed to deserialize ReviewSnapshot from JSONB", e);
        }
    }
}
