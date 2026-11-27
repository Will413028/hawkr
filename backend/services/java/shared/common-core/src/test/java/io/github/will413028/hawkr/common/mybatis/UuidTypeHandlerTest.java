package io.github.will413028.hawkr.common.mybatis;

import org.apache.ibatis.session.Configuration;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.TypeHandler;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class UuidTypeHandlerTest {

    private static final UUID ID = UUID.fromString("0192f3a4-7b1c-7d2e-8f3a-000000000001");

    @Test
    @DisplayName("type-handlers-package 掃到本套件後，UUID 對應到 UuidTypeHandler")
    void packageScanRegistersTheHandlerForUuid() {
        Configuration configuration = new Configuration();
        configuration.getTypeHandlerRegistry().register("io.github.will413028.hawkr.common.mybatis");

        TypeHandler<?> handler = configuration.getTypeHandlerRegistry().getTypeHandler(UUID.class);

        assertThat(handler).isInstanceOf(UuidTypeHandler.class);
    }

    @Test
    @DisplayName("寫入以 setObject 傳 UUID；讀回以 getObject(col, UUID.class)")
    void writesAndReadsTheUuidObject() throws Exception {
        UuidTypeHandler handler = new UuidTypeHandler();
        PreparedStatement ps = mock(PreparedStatement.class);
        handler.setParameter(ps, 1, ID, JdbcType.OTHER);
        verify(ps).setObject(1, ID);

        ResultSet rs = mock(ResultSet.class);
        when(rs.getObject("sku_id", UUID.class)).thenReturn(ID);
        assertThat(handler.getResult(rs, "sku_id")).isEqualTo(ID);
    }

    @Test
    @DisplayName("NULL 讀回 null")
    void nullReadsAsNull() throws Exception {
        UuidTypeHandler handler = new UuidTypeHandler();
        ResultSet rs = mock(ResultSet.class);
        when(rs.getObject("sku_id", UUID.class)).thenReturn(null);
        assertThat(handler.getResult(rs, "sku_id")).isNull();
    }
}
