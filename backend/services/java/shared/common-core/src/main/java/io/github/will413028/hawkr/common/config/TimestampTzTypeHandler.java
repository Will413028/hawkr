package io.github.will413028.hawkr.common.config;

import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.MappedTypes;

import java.sql.*;
import java.time.LocalDateTime;

/**
 * TIMESTAMPTZ ↔ LocalDateTime 型別轉換器。
 * <p>
 * PostgreSQL JDBC driver 42.x 不允許 {@code ResultSet.getObject(idx, LocalDateTime.class)}
 * 讀取 TIMESTAMPTZ 欄位（因 LocalDateTime 無時區資訊）。
 * 本 TypeHandler 使用 {@code getTimestamp().toLocalDateTime()} 繞過此限制。
 * <p>
 * 搭配 {@code connection-init-sql=SET timezone='UTC'}，確保 JDBC session 為 UTC，
 * 讀出的 LocalDateTime 值語意為 UTC。
 */
@MappedTypes(LocalDateTime.class)
public class TimestampTzTypeHandler extends BaseTypeHandler<LocalDateTime> {

    @Override
    public void setNonNullParameter(PreparedStatement ps, int i, LocalDateTime parameter, JdbcType jdbcType)
            throws SQLException {
        ps.setTimestamp(i, Timestamp.valueOf(parameter));
    }

    @Override
    public LocalDateTime getNullableResult(ResultSet rs, String columnName) throws SQLException {
        Timestamp ts = rs.getTimestamp(columnName);
        return ts != null ? ts.toLocalDateTime() : null;
    }

    @Override
    public LocalDateTime getNullableResult(ResultSet rs, int columnIndex) throws SQLException {
        Timestamp ts = rs.getTimestamp(columnIndex);
        return ts != null ? ts.toLocalDateTime() : null;
    }

    @Override
    public LocalDateTime getNullableResult(CallableStatement cs, int columnIndex) throws SQLException {
        Timestamp ts = cs.getTimestamp(columnIndex);
        return ts != null ? ts.toLocalDateTime() : null;
    }
}
