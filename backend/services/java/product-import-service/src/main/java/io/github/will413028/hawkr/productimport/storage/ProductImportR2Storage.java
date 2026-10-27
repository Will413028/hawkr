package io.github.will413028.hawkr.productimport.storage;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.core.ResponseInputStream;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.DeleteObjectRequest;
import software.amazon.awssdk.services.s3.model.GetObjectRequest;
import software.amazon.awssdk.services.s3.model.GetObjectResponse;
import software.amazon.awssdk.services.s3.model.PutObjectRequest;

import java.net.URI;

/**
 * Product Import 專用 R2 儲存（唯一儲存後端）。
 *
 * <p>R2 為 S3 相容 API，以 AWS S3 SDK v2 對 R2 endpoint 操作。objectKey 生成與 local dev
 * fallback 沿用 {@link AbstractProductImportStorage}。
 *
 * <p>{@code hawkr.product-import.storage.backend} 缺省或 {@code r2} 時生效；任何其他值
 * （如遺留的 {@code oss}）不裝配 bean，讓服務啟動即 fail-fast（OSS 遷移已完成、無退路）。
 */
@Service
@ConditionalOnProperty(name = "hawkr.product-import.storage.backend", havingValue = "r2", matchIfMissing = true)
public class ProductImportR2Storage extends AbstractProductImportStorage {

    private static final ObsLogger log = ObsLogger.forClass(ProductImportR2Storage.class);

    @Value("${hawkr.product-import.r2.account-id:}")
    private String accountId;

    @Value("${hawkr.product-import.r2.access-key-id:}")
    private String accessKeyId;

    @Value("${hawkr.product-import.r2.secret-access-key:}")
    private String secretAccessKey;

    @Value("${hawkr.product-import.r2.bucket-name:}")
    private String bucketName;

    private S3Client s3Client;

    @PostConstruct
    public void init() {
        if (accountId != null && !accountId.isBlank()
                && accessKeyId != null && !accessKeyId.isBlank()
                && secretAccessKey != null && !secretAccessKey.isBlank()
                && bucketName != null && !bucketName.isBlank()) {
            URI endpoint = URI.create("https://" + accountId + ".r2.cloudflarestorage.com");
            this.s3Client = S3Client.builder()
                    .endpointOverride(endpoint)
                    .credentialsProvider(StaticCredentialsProvider.create(
                            AwsBasicCredentials.create(accessKeyId, secretAccessKey)))
                    .region(Region.of("auto"))
                    .build();
            log.info("ProductImportR2Storage.init.success", "R2 client ready, bucket=" + bucketName);
        } else {
            log.warn("ProductImportR2Storage.init.skip",
                    "R2 not configured, dev mode — local fallback object key only");
        }
    }

    @PreDestroy
    public void destroy() {
        if (s3Client != null) {
            s3Client.close();
        }
    }

    @Override
    protected boolean enabled() {
        return s3Client != null;
    }

    @Override
    protected void doPut(String objectKey, byte[] content) {
        PutObjectRequest req = PutObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .contentType("text/csv")
                .contentLength((long) content.length)
                .build();
        s3Client.putObject(req, RequestBody.fromBytes(content));
        log.info("ProductImportR2Storage.uploadCsv.success",
                "Uploaded CSV objectKey=" + objectKey + ", size=" + content.length);
    }

    @Override
    protected byte[] doGet(String objectKey) {
        try (ResponseInputStream<GetObjectResponse> in = s3Client.getObject(GetObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .build())) {
            return in.readAllBytes();
        } catch (Exception e) {
            log.error("ProductImportR2Storage.downloadCsv.error",
                    "Failed to download CSV: " + objectKey, e);
            return null;
        }
    }

    @Override
    protected void doDelete(String objectKey) {
        try {
            s3Client.deleteObject(DeleteObjectRequest.builder()
                    .bucket(bucketName)
                    .key(objectKey)
                    .build());
            log.info("ProductImportR2Storage.deleteObject.success", "Deleted: " + objectKey);
        } catch (Exception e) {
            log.warn("ProductImportR2Storage.deleteObject.error",
                    "Failed to delete: " + objectKey + ", reason: " + e.getMessage());
        }
    }

    @Override
    protected void onLocalCache(String op, String objectKey) {
        log.warn("ProductImportR2Storage." + op + ".localCache",
                "R2 disabled, using in-memory fallback: " + objectKey);
    }
}
