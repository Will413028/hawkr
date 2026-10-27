package io.github.will413028.hawkr.productimport.storage;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.DeleteObjectRequest;
import software.amazon.awssdk.services.s3.model.PutObjectRequest;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

/**
 * {@link ProductImportR2Storage} 單元測試（mock S3 client 層級）。
 */
class ProductImportR2StorageTest {

    private static final String BUCKET = "hawkr-media-dev";

    private S3Client s3Client;
    private ProductImportR2Storage storage;

    @BeforeEach
    void setUp() {
        s3Client = mock(S3Client.class);
        storage = new ProductImportR2Storage();
        ReflectionTestUtils.setField(storage, "tempPrefix", "tmp");
        ReflectionTestUtils.setField(storage, "bucketName", BUCKET);
        ReflectionTestUtils.setField(storage, "s3Client", s3Client);
    }

    @Test
    @DisplayName("uploadCsv：objectKey 結構 tmp/product-import/{store}/{job}.csv 並 putObject")
    void uploadCsvKeyAndPut() {
        String key = storage.uploadCsv(100L, 200L, "a,b\n".getBytes());
        assertEquals("tmp/product-import/100/200.csv", key);

        ArgumentCaptor<PutObjectRequest> captor = ArgumentCaptor.forClass(PutObjectRequest.class);
        verify(s3Client).putObject(captor.capture(), any(RequestBody.class));
        assertEquals(BUCKET, captor.getValue().bucket());
        assertEquals(key, captor.getValue().key());
        assertEquals("text/csv", captor.getValue().contentType());
    }

    @Test
    @DisplayName("deleteObject：正確 key 傳遞")
    void deleteKey() {
        storage.deleteObject("tmp/product-import/100/200.csv");
        ArgumentCaptor<DeleteObjectRequest> captor = ArgumentCaptor.forClass(DeleteObjectRequest.class);
        verify(s3Client).deleteObject(captor.capture());
        assertEquals("tmp/product-import/100/200.csv", captor.getValue().key());
    }

    @Test
    @DisplayName("未配置（dev）：走 local cache fallback，回傳虛擬 key 並可重抓")
    void localCacheFallbackWhenDisabled() {
        ProductImportR2Storage bare = new ProductImportR2Storage();
        ReflectionTestUtils.setField(bare, "tempPrefix", "tmp");
        assertFalse(bare.enabled());

        byte[] content = "x,y\n".getBytes();
        String key = bare.uploadCsv(1L, 2L, content);
        assertEquals("tmp/product-import/1/2.csv", key);
        assertArrayEquals(content, bare.downloadCsv(key));
    }
}
