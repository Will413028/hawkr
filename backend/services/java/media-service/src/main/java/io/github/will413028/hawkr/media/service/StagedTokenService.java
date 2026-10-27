package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.StagedTokenScope;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.jsonwebtoken.*;
import io.jsonwebtoken.security.Keys;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.Arrays;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.Date;
import java.util.UUID;

/**
 * Staged Token 服務
 * <p>
 * 負責生成與驗證 JWT 格式的 Staged Token Token 規格依設計書 3.2.1
 */
@Service
public class StagedTokenService {

    private static final ObsLogger log = ObsLogger.forClass(StagedTokenService.class);

    private static final String CLAIM_SESSION_ID = "session_id";
    private static final String CLAIM_ACCOUNT_ID = "account_id";
    private static final String CLAIM_IS_SECURE = "is_secure";
    private static final String CLAIM_SCOPE = "scope";

    /**
     * 簽章祕鑰。**無明文 fallback**（#1396）：祕鑰必由 Secret 注入 {@code TOKEN_SECRET}；
     * 缺失時啟動即 fail-closed（deny-by-default），不依賴 active profile。
     * 唯一例外由 {@code media.token.allow-ephemeral=true} 顯式開啟（僅 local/test 設），
     * 才以 ephemeral 隨機 key 運作，絕不與 production 共享 default。
     */
    @Value("${media.token.secret:}")
    private String tokenSecret;

    @Value("${media.token.issuer:media-service}")
    private String tokenIssuer;

    /**
     * 是否允許缺 secret 時以 ephemeral 隨機 key 啟動。**default false = fail-closed**；
     * 只有 local/test 顯式設 {@code true} 才放行，任何未設此旗標的環境（含 prod）缺 secret 即拒絕啟動。
     */
    @Value("${media.token.allow-ephemeral:false}")
    private boolean allowEphemeral;

    private SecretKey secretKey;

    @PostConstruct
    public void init() {
        if (tokenSecret != null && !tokenSecret.isBlank()) {
            this.secretKey = Keys.hmacShaKeyFor(tokenSecret.getBytes(StandardCharsets.UTF_8));
            return;
        }
        // 缺 key：deny-by-default，除非顯式允許 ephemeral，否則一律拒絕啟動（fail-closed）
        if (!allowEphemeral) {
            throw new IllegalStateException(
                "media.token.secret（TOKEN_SECRET）未提供：拒絕啟動，"
                    + "祕鑰必須由 Secret 注入，禁止明文 fallback；"
                    + "如為 local/test 請顯式設 media.token.allow-ephemeral=true（#1396）");
        }
        // 顯式允許（僅 local/test）：生成 ephemeral 隨機 key，不與 production 共享
        byte[] ephemeral = new byte[64];
        new SecureRandom().nextBytes(ephemeral);
        this.secretKey = Keys.hmacShaKeyFor(ephemeral);
        Arrays.fill(ephemeral, (byte) 0);
        log.warn("token.secret.ephemeral",
            "media.token.secret 未設定，使用 ephemeral 隨機 key（僅限 local/test；服務重啟後既有 token 失效）");
    }

    /**
     * 生成 Staged Token
     *
     * @param sessionId 會話 ID
     * @param accountId 帳號 ID
     * @param isSecure 是否為機敏檔案
     * @param expiresAt 過期時間
     * @return JWT Token
     */
    public String generateToken(UUID sessionId, Long accountId, Boolean isSecure, LocalDateTime expiresAt) {
        String scope = Boolean.TRUE.equals(isSecure)
                ? StagedTokenScope.MEDIA_SECURE_UPLOAD.value()
                : StagedTokenScope.MEDIA_UPLOAD_SESSION.value();

        Date expiration = new Date(DateTimeUtils.toEpochMillis(expiresAt));

        String token = Jwts.builder()
                .issuer(tokenIssuer)
                .claim(CLAIM_SESSION_ID, sessionId.toString())
                .claim(CLAIM_ACCOUNT_ID, accountId)
                .claim(CLAIM_IS_SECURE, isSecure)
                .claim(CLAIM_SCOPE, scope)
                .expiration(expiration)
                .issuedAt(new Date())
                .signWith(secretKey)
                .compact();

        log.info("token.generated", "生成 Staged Token", "session_id", sessionId.toString());
        return token;
    }

    /**
     * 解析並驗證 Staged Token
     *
     * @param token JWT Token
     * @return Token Claims
     * @throws MediaServiceException 如果 Token 無效或已過期
     */
    public StagedTokenClaims parseAndValidate(String token) {
        try {
            Claims claims = Jwts.parser()
                    .verifyWith(secretKey)
                    .requireIssuer(tokenIssuer)
                    .build()
                    .parseSignedClaims(token)
                    .getPayload();

            String sessionIdStr = claims.get(CLAIM_SESSION_ID, String.class);
            Long accountId = claims.get(CLAIM_ACCOUNT_ID, Long.class);
            Boolean isSecure = claims.get(CLAIM_IS_SECURE, Boolean.class);
            String scope = claims.get(CLAIM_SCOPE, String.class);
            Date expiration = claims.getExpiration();

            if (sessionIdStr == null || accountId == null) {
                throw MediaServiceException.invalidArgument("Token claims 缺失必要欄位");
            }

            LocalDateTime expiresAt = DateTimeUtils.fromEpochMillis(expiration.getTime());

            return StagedTokenClaims.builder()
                    .sessionId(UUID.fromString(sessionIdStr))
                    .accountId(accountId)
                    .isSecure(Boolean.TRUE.equals(isSecure))
                    .scope(scope)
                    .expiresAt(expiresAt)
                    .build();

        } catch (ExpiredJwtException e) {
            log.warn("token.expired", "Token 已過期");
            throw MediaServiceException.invalidArgument("Token 已過期");
        } catch (JwtException e) {
            log.warn("token.invalid", "Token 驗證失敗: " + e.getMessage());
            throw MediaServiceException.invalidArgument("Token 驗證失敗: " + e.getMessage());
        } catch (IllegalArgumentException e) {
            log.warn("token.parse.failed", "Token 解析失敗: " + e.getMessage());
            throw MediaServiceException.invalidArgument("Token 解析失敗: " + e.getMessage());
        }
    }
}
