package io.github.will413028.hawkr.productimport.storage;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Product Import 儲存後端條件裝配測試（49 號 §8.5）。
 * OSS 遷移完成後唯一實作為 R2；缺省或 {@code r2} 生效，任何其他值不裝配（fail-fast）。
 */
class ProductImportStorageBackendConfigTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
        .withUserConfiguration(ProductImportR2Storage.class);

    @Test
    @DisplayName("預設（未設 backend）→ R2 bean 生效")
    void defaultIsR2() {
        runner.run(context -> {
            assertThat(context).hasSingleBean(ProductImportStorage.class);
            assertThat(context.getBean(ProductImportStorage.class))
                .isInstanceOf(ProductImportR2Storage.class);
        });
    }

    @Test
    @DisplayName("backend=r2 → R2 bean 生效")
    void explicitR2() {
        runner.withPropertyValues("hawkr.product-import.storage.backend=r2").run(context -> {
            assertThat(context).hasSingleBean(ProductImportStorage.class);
            assertThat(context.getBean(ProductImportStorage.class))
                .isInstanceOf(ProductImportR2Storage.class);
        });
    }

    @Test
    @DisplayName("backend=oss（遺留值）→ 不裝配任何 ProductImportStorage bean（fail-fast）")
    void legacyOssNotWired() {
        runner.withPropertyValues("hawkr.product-import.storage.backend=oss").run(context -> {
            assertThat(context).doesNotHaveBean(ProductImportStorage.class);
        });
    }
}
