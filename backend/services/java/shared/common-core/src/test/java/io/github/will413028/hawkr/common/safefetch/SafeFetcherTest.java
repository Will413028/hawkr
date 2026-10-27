package io.github.will413028.hawkr.common.safefetch;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.net.InetAddress;
import java.net.UnknownHostException;
import java.net.http.HttpClient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * SafeFetcher 黑箱測試集（依 40 號設計書 §10.3 必涵蓋 10 條 SSRF case）。
 *
 * <p>本測試僅驗證「validateUrl + IP 黑名單 + redirect 語意」純函式邏輯，
 * 不對 internet 真實打 HTTP（避免測試環境依賴 + flakiness）。</p>
 *
 * <p>SafeFetcherImpl 把 parseAndValidateUrl / validateHostnameAndResolveIps / checkIp
 * 方法設為 package-private 即為支援黑箱測試（測試可見度放寬，非疏漏）。</p>
 */
class SafeFetcherTest {

    private final SafeFetcherImpl impl = new SafeFetcherImpl(HttpClient.newHttpClient());

    @Test
    @DisplayName("§10.3 case 1: http:// 協定拒絕")
    void rejectHttpProtocol() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.parseAndValidateUrl("http://example.com/x.jpg"));
        assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 2: file:// 協定拒絕")
    void rejectFileProtocol() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.parseAndValidateUrl("file:///etc/passwd"));
        assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 2b: javascript: / data: / ftp: 等均拒絕")
    void rejectMaliciousProtocols() {
        for (String url : new String[]{
            "javascript:alert(1)",
            "data:text/html,<script>x</script>",
            "ftp://example.com/foo",
            "gopher://example.com/_select%20*",
            "dict://attacker.com/x"
        }) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.parseAndValidateUrl(url));
            assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex.reason(),
                "URL should be rejected: " + url);
        }
    }

    @Test
    @DisplayName("§10.3 case 3: localhost hostname 拒絕")
    void rejectLocalhostHostname() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.validateHostnameAndResolveIps("localhost"));
        assertEquals(SafeFetchException.Reason.DOMAIN_BLOCKED_INTERNAL, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 4: AWS metadata 169.254.169.254 拒絕")
    void rejectAwsMetadataIp() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.validateHostnameAndResolveIps("169.254.169.254"));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_LINK_LOCAL, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 5: 10.x 私網拒絕")
    void rejectPrivate10Ip() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.validateHostnameAndResolveIps("10.0.0.1"));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 5b: 192.168.x / 172.16.x 私網拒絕")
    void rejectOtherPrivateRanges() {
        for (String ip : new String[]{"192.168.1.1", "172.16.0.1", "172.31.255.254"}) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.validateHostnameAndResolveIps(ip),
                "IP should be rejected as private: " + ip);
            assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason(),
                "Reason mismatch for " + ip);
        }
    }

    @Test
    @DisplayName("§10.3 case 6: *.example.com 自家網域拒絕")
    void rejectInternalDomain() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.validateHostnameAndResolveIps("internal.example.com"));
        assertEquals(SafeFetchException.Reason.DOMAIN_BLOCKED_INTERNAL, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 6b: *.internal / *.svc / *.cluster.local k8s hostname 拒絕")
    void rejectKubernetesHostnames() {
        for (String host : new String[]{
            "product-service.internal",
            "media.svc",
            "redis-master.hawkr.svc.cluster.local"
        }) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.validateHostnameAndResolveIps(host));
            assertEquals(SafeFetchException.Reason.DOMAIN_BLOCKED_INTERNAL, ex.reason(),
                "Hostname should be blocked: " + host);
        }
    }

    @Test
    @DisplayName("§10.3 case 7: 127.0.0.1 loopback 拒絕")
    void rejectLoopbackIp() {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.validateHostnameAndResolveIps("127.0.0.1"));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_LOOPBACK, ex.reason());
    }

    @Test
    @DisplayName("§10.3 case 9: 跨域 redirect 偵測（resolveRelativeLocation 邏輯）")
    void detectCrossDomainRedirect() {
        String resolved = SafeFetcherImpl.resolveRelativeLocation(
            "https://a.example.com/foo", "https://b.attacker.com/bar");
        assertTrue(resolved.startsWith("https://b.attacker.com/"));
        // 實際 cross-domain 比對在 fetch() 主 loop 內：URI host 不一致 → REDIRECT_CROSS_DOMAIN
    }

    @Test
    @DisplayName("§10.1 守則 11: Content-Type 白名單正規化（image/jpeg; charset=utf-8 → image/jpeg）")
    void normalizeContentType() {
        assertEquals("image/jpeg", SafeFetcherImpl.normalizeContentType("image/jpeg; charset=utf-8"));
        assertEquals("text/html", SafeFetcherImpl.normalizeContentType("Text/HTML"));
        assertEquals("", SafeFetcherImpl.normalizeContentType(null));
    }

    @Test
    @DisplayName("§10.1 守則 1: scheme 大小寫不敏感 + URI 結構不合法時拋 PROTOCOL_NOT_ALLOWED")
    void handleBadUri() {
        // 大小寫 https
        SafeFetchException ex1 = assertThrows(SafeFetchException.class,
            () -> impl.parseAndValidateUrl("HTTP://example.com/x"));
        assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex1.reason());

        // 空字串
        SafeFetchException ex2 = assertThrows(SafeFetchException.class,
            () -> impl.parseAndValidateUrl(""));
        assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex2.reason());

        // null
        SafeFetchException ex3 = assertThrows(SafeFetchException.class,
            () -> impl.parseAndValidateUrl(null));
        assertEquals(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED, ex3.reason());
    }

    @Test
    @DisplayName("#1399: 100.64.0.0/10 CGNAT 拒絕（含阿里雲 metadata 100.100.100.200）")
    void rejectCgnatRange() throws UnknownHostException {
        for (String ip : new String[]{"100.64.0.1", "100.100.100.200", "100.127.255.254"}) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.checkIp(InetAddress.getByName(ip)),
                "CGNAT IP should be blocked: " + ip);
            assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason(), ip);
        }
        // 邊界外 100.63.x / 100.128.x 應允許（非 CGNAT）
        impl.checkIp(InetAddress.getByName("100.63.255.255"));
        impl.checkIp(InetAddress.getByName("100.128.0.1"));
    }

    @Test
    @DisplayName("#1399: 255.255.255.255 廣播 + 240.0.0.0/4 class E 拒絕")
    void rejectBroadcastAndClassE() throws UnknownHostException {
        for (String ip : new String[]{"255.255.255.255", "240.0.0.1", "250.1.2.3"}) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.checkIp(InetAddress.getByName(ip)),
                "reserved IP should be blocked: " + ip);
            assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason(), ip);
        }
    }

    @Test
    @DisplayName("#1399: IPv4-mapped IPv6 ::ffff:169.254.169.254 還原後仍拒絕 metadata")
    void rejectIpv4MappedMetadata() throws UnknownHostException {
        SafeFetchException ex = assertThrows(SafeFetchException.class,
            () -> impl.checkIp(InetAddress.getByName("::ffff:169.254.169.254")));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_LINK_LOCAL, ex.reason());
    }

    @Test
    @DisplayName("#1399: NAT64 64:ff9b::/96 內嵌 10.0.0.1 還原後拒絕私網")
    void rejectNat64EmbeddedPrivate() throws UnknownHostException {
        // 64:ff9b:0:0:0:0:10.0.0.1 → Inet6Address（非 mapped），checkIp 應解出 10.0.0.1
        byte[] nat64 = new byte[]{
            0x00, 0x64, (byte) 0xff, (byte) 0x9b,
            0, 0, 0, 0, 0, 0, 0, 0,
            10, 0, 0, 1
        };
        InetAddress addr = InetAddress.getByAddress(nat64);
        SafeFetchException ex = assertThrows(SafeFetchException.class, () -> impl.checkIp(addr));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason());
    }

    @Test
    @DisplayName("#1399: IPv6 loopback ::1 + ULA fc00::/7 拒絕")
    void rejectIpv6LoopbackAndUla() throws UnknownHostException {
        SafeFetchException lo = assertThrows(SafeFetchException.class,
            () -> impl.checkIp(InetAddress.getByName("::1")));
        assertEquals(SafeFetchException.Reason.IP_BLOCKED_LOOPBACK, lo.reason());
        for (String ip : new String[]{"fc00::1", "fd12:3456::1"}) {
            SafeFetchException ex = assertThrows(SafeFetchException.class,
                () -> impl.checkIp(InetAddress.getByName(ip)), ip);
            assertEquals(SafeFetchException.Reason.IP_BLOCKED_PRIVATE, ex.reason(), ip);
        }
    }

    @Test
    @DisplayName("#1399: 公網 IPv4 通過（不誤擋）")
    void allowPublicIpv4() throws UnknownHostException {
        // 8.8.8.8 為 literal，getByName 不查 DNS；不應拋例外
        impl.checkIp(InetAddress.getByName("8.8.8.8"));
        impl.checkIp(InetAddress.getByName("93.184.216.34"));
    }

    @Test
    @DisplayName("isIpLiteral 純函式：IPv4 / IPv6 / hostname 判斷")
    void isIpLiteralUtility() {
        assertTrue(SafeFetcherImpl.isIpLiteral("10.0.0.1"));
        assertTrue(SafeFetcherImpl.isIpLiteral("169.254.169.254"));
        assertTrue(SafeFetcherImpl.isIpLiteral("::1"));
        assertTrue(SafeFetcherImpl.isIpLiteral("fe80::1"));
        assertEquals(false, SafeFetcherImpl.isIpLiteral("example.com"));
        assertEquals(false, SafeFetcherImpl.isIpLiteral("foo.bar.baz"));
    }
}
