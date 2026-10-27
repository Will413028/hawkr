package io.github.will413028.hawkr.media.storage;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 儲存後端條件裝配測試（49 號 §8.5）：{@code hawkr.media.storage.backend}。
 * OSS 遷移完成後唯一實作為 R2；缺省或 {@code r2} 生效，任何其他值不裝配（fail-fast）。
 */
class StorageBackendConfigTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
        .withUserConfiguration(R2StorageService.class);

    @Test
    @DisplayName("預設（未設 backend）→ R2 bean 生效")
    void defaultIsR2() {
        runner.run(context -> {
            assertThat(context).hasSingleBean(StorageService.class);
            assertThat(context.getBean(StorageService.class)).isInstanceOf(R2StorageService.class);
        });
    }

    @Test
    @DisplayName("backend=r2 → R2 bean 生效")
    void explicitR2() {
        runner.withPropertyValues("hawkr.media.storage.backend=r2").run(context -> {
            assertThat(context).hasSingleBean(StorageService.class);
            assertThat(context.getBean(StorageService.class)).isInstanceOf(R2StorageService.class);
        });
    }

    @Test
    @DisplayName("backend=oss（遺留值）→ 不裝配任何 StorageService bean（fail-fast）")
    void legacyOssNotWired() {
        runner.withPropertyValues("hawkr.media.storage.backend=oss").run(context -> {
            assertThat(context).doesNotHaveBean(StorageService.class);
        });
    }
}
