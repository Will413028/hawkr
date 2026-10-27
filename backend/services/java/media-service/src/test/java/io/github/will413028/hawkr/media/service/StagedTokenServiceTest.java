package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.StagedTokenScope;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.Date;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * {@link StagedTokenService} 測試（#1396 fail-closed + 簽章完整性）。
 */
class StagedTokenServiceTest {

    private static final String SECRET = "test-staged-token-secret-key-min-32-bytes-long";
    private static final String ISSUER = "media-service";

    private StagedTokenService newService(String secret, boolean allowEphemeral) {
        StagedTokenService service = new StagedTokenService();
        ReflectionTestUtils.setField(service, "tokenSecret", secret);
        ReflectionTestUtils.setField(service, "tokenIssuer", ISSUER);
        ReflectionTestUtils.setField(service, "allowEphemeral", allowEphemeral);
        return service;
    }

    @Test
    @DisplayName("#1396 fail-closed：缺 secret 且未允許 ephemeral → 啟動拋出，拒絕服務（deny-by-default）")
    void missingSecretFailsClosed() {
        StagedTokenService service = newService("", false);
        assertThrows(IllegalStateException.class, service::init);
    }

    @Test
    @DisplayName("#1396：缺 secret 但顯式 allow-ephemeral=true → ephemeral key，generate/parse 可運作")
    void missingSecretWithAllowEphemeralUsesEphemeralKey() {
        StagedTokenService service = newService("", true);
        service.init();

        UUID sessionId = UUID.randomUUID();
        LocalDateTime expires = DateTimeUtils.nowUtc().plusMinutes(30);
        String token = service.generateToken(sessionId, 12345L, false, expires);
        assertNotNull(token);

        StagedTokenClaims claims = service.parseAndValidate(token);
        assertEquals(sessionId, claims.getSessionId());
        assertEquals(12345L, claims.getAccountId());
        assertEquals(StagedTokenScope.MEDIA_UPLOAD_SESSION.value(), claims.getScope());
    }

    @Test
    @DisplayName("明確 secret：generate/parse round-trip 一致（含 secure scope）")
    void explicitSecretRoundTrip() {
        StagedTokenService service = newService(SECRET, false);
        service.init();

        UUID sessionId = UUID.randomUUID();
        LocalDateTime expires = DateTimeUtils.nowUtc().plusMinutes(30);
        String token = service.generateToken(sessionId, 999L, true, expires);

        StagedTokenClaims claims = service.parseAndValidate(token);
        assertEquals(999L, claims.getAccountId());
        assertEquals(true, claims.getIsSecure());
        assertEquals(StagedTokenScope.MEDIA_SECURE_UPLOAD.value(), claims.getScope());
    }

    @Test
    @DisplayName("偽造簽章（不同 key）→ parseAndValidate 拒絕")
    void forgedSignatureRejected() {
        StagedTokenService service = newService(SECRET, false);
        service.init();

        SecretKey attackerKey = Keys.hmacShaKeyFor(
            "attacker-forged-key-completely-different-32b".getBytes(StandardCharsets.UTF_8));
        String forged = Jwts.builder()
            .issuer(ISSUER)
            .claim("session_id", UUID.randomUUID().toString())
            .claim("account_id", 12345L)
            .expiration(new Date(System.currentTimeMillis() + 60_000))
            .issuedAt(new Date())
            .signWith(attackerKey)
            .compact();

        assertThrows(MediaServiceException.class, () -> service.parseAndValidate(forged));
    }

    @Test
    @DisplayName("key rotation：舊 key 簽的 token 用新 key parse → 拒絕（換 secret 後既有 token 失效）")
    void tokenSignedWithOldKeyRejectedAfterRotation() {
        // 舊 key 簽發 token
        StagedTokenService oldService = newService("old-staged-token-secret-key-min-32-bytes-long", false);
        oldService.init();
        UUID sessionId = UUID.randomUUID();
        String token = oldService.generateToken(
            sessionId, 12345L, false, DateTimeUtils.nowUtc().plusMinutes(30));

        // 新 key（rotation 後）parse 同一 token
        StagedTokenService newService = newService("new-staged-token-secret-key-min-32-bytes-long", false);
        newService.init();

        assertThrows(MediaServiceException.class, () -> newService.parseAndValidate(token));
    }

    @Test
    @DisplayName("過期 token → parseAndValidate 拒絕")
    void expiredTokenRejected() {
        StagedTokenService service = newService(SECRET, false);
        service.init();

        UUID sessionId = UUID.randomUUID();
        LocalDateTime expired = DateTimeUtils.nowUtc().minusMinutes(1);
        String token = service.generateToken(sessionId, 12345L, false, expired);

        assertThrows(MediaServiceException.class, () -> service.parseAndValidate(token));
    }
}
