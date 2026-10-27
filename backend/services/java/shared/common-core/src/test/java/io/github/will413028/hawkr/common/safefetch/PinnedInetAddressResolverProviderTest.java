package io.github.will413028.hawkr.common.safefetch;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.net.InetAddress;
import java.net.UnknownHostException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * #1399 DNS rebinding pin 機制測試（resolver SPI 層）。
 *
 * <p>驗證 {@link PinnedInetAddressResolverProvider} 已被 JVM 全域安裝：pin 的 hostname 在 DNS
 * 解析（即 HttpClient 內部連線時的第二次 resolve）命中 pin 時直接回傳已驗 IP，而非查 DNS；未 pin
 * 者一律 delegate builtin resolver。用 RFC 6761 保留的 {@code .invalid} TLD + 每測唯一名稱，
 * 避開 JDK 正/負向 DNS 快取干擾，確保斷言確定。</p>
 *
 * <p>註：JDK 的 InetAddress 正向快取層位於 resolver SPI 之上。真實 SafeFetch 流程中，
 * {@code validateHostnameAndResolveIps} 先解析並以「已驗 IP」seed 快取，接著 pin，HttpClient
 * 連線時的 resolve 命中快取（已驗 IP）或（快取關閉時）命中 resolver 的 pin — 兩路皆回已驗 IP，
 * 因此對快取開/關組態都 fail-closed。本測試以「pin 前從未解析」的唯一 host 驗證 resolver 層的
 * pin 行為（即快取關閉時的兜底路徑）。</p>
 */
class PinnedInetAddressResolverProviderTest {

    /** 公網 IP literal（getByName 不查 DNS），僅作為 pin 目標值。 */
    private static final String PUBLIC_IP = "93.184.216.34";

    private static String uniqueInvalidHost() {
        return "safefetch-pin-" + System.nanoTime() + ".invalid";
    }

    @Test
    @DisplayName("pin 後解析回傳已驗 IP（不查 DNS）— 消滅 rebinding 第二次 resolve 窗口")
    void pinnedHostResolvesToPinnedIp() throws UnknownHostException {
        String host = uniqueInvalidHost();
        InetAddress pinned = InetAddress.getByName(PUBLIC_IP);
        PinnedInetAddressResolverProvider.pin(host, new InetAddress[]{pinned});
        try {
            InetAddress[] resolved = InetAddress.getAllByName(host);
            assertEquals(1, resolved.length);
            assertEquals(PUBLIC_IP, resolved[0].getHostAddress(),
                "pin 命中時必須回傳已驗 IP，而非 DNS 結果");
        } finally {
            PinnedInetAddressResolverProvider.unpin(host);
        }
    }

    @Test
    @DisplayName("unpin 後解析不再回傳 pin 值 — pin 不殘留（唯一 host 避開快取）")
    void unpinRemovesPin() {
        String host = uniqueInvalidHost();
        // pin 後立即 unpin，中間不解析 → 無 JDK 正向快取 → 之後解析走 resolver（無 pin）→ builtin
        PinnedInetAddressResolverProvider.pin(host, new InetAddress[]{safeByName(PUBLIC_IP)});
        PinnedInetAddressResolverProvider.unpin(host);
        try {
            InetAddress[] resolved = InetAddress.getAllByName(host);
            assertNotEquals(PUBLIC_IP, resolved[0].getHostAddress(),
                "unpin 後不得再回傳 pin 值（證明已回落 builtin）");
        } catch (UnknownHostException expectedWhenNxdomain) {
            // .invalid 標準環境為 NXDOMAIN，同樣證明已回落 builtin
        }
    }

    @Test
    @DisplayName("未 pin 的 hostname 委派 builtin，不影響其他服務 DNS（provider 透明）")
    void unrelatedHostUnaffected() throws UnknownHostException {
        InetAddress[] resolved = InetAddress.getAllByName("localhost");
        assertTrue(resolved.length > 0);
        assertTrue(resolved[0].isLoopbackAddress());
    }

    @Test
    @DisplayName("pin 大小寫不敏感")
    void pinIsCaseInsensitive() throws UnknownHostException {
        String base = uniqueInvalidHost();
        String upper = base.toUpperCase(java.util.Locale.ROOT);
        PinnedInetAddressResolverProvider.pin(upper, new InetAddress[]{safeByName(PUBLIC_IP)});
        try {
            InetAddress[] resolved = InetAddress.getAllByName(base);
            assertEquals(PUBLIC_IP, resolved[0].getHostAddress());
        } finally {
            PinnedInetAddressResolverProvider.unpin(base);
        }
    }

    private static InetAddress safeByName(String ipLiteral) {
        try {
            return InetAddress.getByName(ipLiteral);
        } catch (UnknownHostException e) {
            throw new IllegalStateException(e);
        }
    }
}
