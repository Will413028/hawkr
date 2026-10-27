package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.media.dto.request.CreateUploadSessionRequest;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code UploadSessionService.createSession} 的原始檔名持久化（issue #3149，設計書 08 §4.9）。
 *
 * <p>W1 的起點：使用者宣告的 {@code filename} 在此之前唯一的用途是取副檔名，建立 session
 * 時即丟棄——於是讀取端只剩 objectKey 尾段（一串 uuid）可用。
 *
 * <p>🔴 <b>兩個去處必須分開</b>（§4.9.3〈作用範圍〉）：{@code generateObjectKey} 吃<b>原值</b>，
 * 只有 {@code original_filename} 走消毒。順序對調會讓 {@code sanitizedExtension} 對路徑穿越
 * 的既有拒絕被靜默關掉——{@code photo.jp/g} 會先被洗成合法的 {@code g}。
 */
class UploadSessionServiceTest {

    private UploadSessionRepository uploadSessionRepository;
    private StorageService storageService;
    private UploadSessionService service;

    private static final Long ACCOUNT_ID = 2600000042L;

    @BeforeEach
    void setUp() {
        uploadSessionRepository = mock(UploadSessionRepository.class);
        storageService = mock(StorageService.class);
        service = new UploadSessionService(
                uploadSessionRepository,
                storageService,
                mock(StagedTokenService.class),
                mock(MediaUploadValidator.class));
    }

    private static CreateUploadSessionRequest request(String filename) {
        return CreateUploadSessionRequest.builder()
                .filename(filename)
                .contentType("application/pdf")
                .fileSize(10240L)
                .isSecure(true)
                .build();
    }

    private UploadSessionEntity captureInsertedSession() {
        ArgumentCaptor<UploadSessionEntity> captor = ArgumentCaptor.forClass(UploadSessionEntity.class);
        verify(uploadSessionRepository).insert(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("createSession 把消毒後的原始檔名寫進 upload_session")
    void persistsSanitizedOriginalFilename() {
        when(storageService.generateObjectKey(anyString(), anyBoolean()))
                .thenReturn("tmp/20260905/9f1c2b3a.pdf");

        // 含路徑 ＋ CRLF ＋ NUL：三種消毒都會動到它，乾淨的短檔名在這裡是零鑑別力的
        service.createSession(ACCOUNT_ID, request("C:\\Users\\user\\稅籍證明\r\n\u0000.pdf"));

        assertThat(captureInsertedSession().getOriginalFilename()).isEqualTo("稅籍證明.pdf");
    }

    @Test
    @DisplayName("§4.9.3〈作用範圍〉：generateObjectKey 拿到的是**原值**，不是消毒後的值")
    void objectKeyGenerationReceivesRawFilename() {
        when(storageService.generateObjectKey(anyString(), anyBoolean()))
                .thenReturn("tmp/20260905/9f1c2b3a.pdf");
        String raw = "C:\\Users\\user\\稅籍證明\r\n\u0000.pdf";

        service.createSession(ACCOUNT_ID, request(raw));

        // 🔴 順序不可對調：sanitize 的第一步是取 basename，會把 `photo.jp/g` 洗成合法的 `g`，
        // 而 sanitizedExtension 對含 / \ .. 的副檔名是丟 INVALID_PARAM 的既有路徑穿越拒絕。
        // 先消毒再生成 key ⇒ 那道拒絕被關掉，而編譯、測試、CI 全綠。
        ArgumentCaptor<String> keyInput = ArgumentCaptor.forClass(String.class);
        verify(storageService).generateObjectKey(keyInput.capture(), anyBoolean());
        assertThat(keyInput.getValue()).isEqualTo(raw);
        assertThat(keyInput.getValue()).isNotEqualTo("稅籍證明.pdf");
    }

    @Test
    @DisplayName("消毒後為空的檔名 → 存 NULL（讓讀取端走回退，不存空字串）")
    void storesNullWhenNothingSurvivesSanitization() {
        when(storageService.generateObjectKey(anyString(), anyBoolean()))
                .thenReturn("tmp/20260905/9f1c2b3a.bin");

        service.createSession(ACCOUNT_ID, request("   "));

        // 空字串與 NULL 在這裡語意相同（「沒有可用的原始檔名」），只留一種表示法
        assertThat(captureInsertedSession().getOriginalFilename()).isNull();
    }

    @Test
    @DisplayName("乾淨的中文檔名原樣落庫")
    void keepsCleanCjkFilename() {
        when(storageService.generateObjectKey(anyString(), anyBoolean()))
                .thenReturn("tmp/20260905/9f1c2b3a.pdf");

        service.createSession(ACCOUNT_ID, request("營業登記證 2026.pdf"));

        assertThat(captureInsertedSession().getOriginalFilename()).isEqualTo("營業登記證 2026.pdf");
    }
}
