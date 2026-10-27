package io.github.will413028.hawkr.product.integration;

import io.github.will413028.hawkr.product.repository.StoreExposureRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@code store_exposure} 的版本條件 upsert 在真實 PostgreSQL 上的行為。
 */
class StoreExposureUpsertIT extends AbstractProductPostgresIT {

    private static final long STORE_ID = 2685914682L;
    private static final long OTHER_STORE_ID = 2632049831L;

    @Autowired StoreExposureRepository repository;
    @Autowired JdbcTemplate jdbc;

    @BeforeEach
    void clean() {
        jdbc.update("DELETE FROM store_exposure");
    }

    private Map<String, Object> row(long storeId) {
        return jdbc.queryForMap("SELECT exposable, version FROM store_exposure WHERE store_id = ?", storeId);
    }

    @Test
    @DisplayName("沒有列：isExposable 為 false")
    void missingRowIsNotExposable() {
        assertThat(repository.isExposable(STORE_ID)).isFalse();
    }

    @Test
    @DisplayName("版本 5 寫入後收到版本 3 → 列不變；收到版本 5 重送 → 不變；版本 6 → 更新")
    void versionRules() {
        assertThat(repository.upsertIfNewer(STORE_ID, true, 5L)).isEqualTo(1);
        assertThat(row(STORE_ID)).containsEntry("exposable", true).containsEntry("version", 5L);

        assertThat(repository.upsertIfNewer(STORE_ID, false, 3L)).isZero();
        assertThat(row(STORE_ID)).containsEntry("exposable", true).containsEntry("version", 5L);

        assertThat(repository.upsertIfNewer(STORE_ID, false, 5L)).isZero();
        assertThat(repository.findVersion(STORE_ID)).isEqualTo(5L);
        assertThat(row(STORE_ID)).containsEntry("exposable", true).containsEntry("version", 5L);

        assertThat(repository.upsertIfNewer(STORE_ID, false, 6L)).isEqualTo(1);
        assertThat(row(STORE_ID)).containsEntry("exposable", false).containsEntry("version", 6L);
        assertThat(repository.isExposable(STORE_ID)).isFalse();
    }

    @Test
    @DisplayName("首次寫入 exposable=true 後 isExposable 為 true；版本只在同一家店之間比較")
    void versionsAreScopedPerStore() {
        assertThat(repository.upsertIfNewer(STORE_ID, true, 9L)).isEqualTo(1);
        assertThat(repository.upsertIfNewer(OTHER_STORE_ID, true, 1L)).isEqualTo(1);

        assertThat(repository.isExposable(STORE_ID)).isTrue();
        assertThat(repository.isExposable(OTHER_STORE_ID)).isTrue();
        assertThat(row(OTHER_STORE_ID)).containsEntry("version", 1L);
    }

    @Test
    @DisplayName("product_suspension_snapshot 已刪除")
    void suspensionSnapshotTableIsDropped() {
        Integer count = jdbc.queryForObject(
                "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'product_suspension_snapshot'",
                Integer.class);
        assertThat(count).isZero();
    }
}
