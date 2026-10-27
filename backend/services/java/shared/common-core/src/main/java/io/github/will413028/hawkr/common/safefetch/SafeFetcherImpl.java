package io.github.will413028.hawkr.common.safefetch;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.Inet4Address;
import java.net.Inet6Address;
import java.net.InetAddress;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.UnknownHostException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.util.Arrays;
import java.util.List;
import java.util.Locale;
import java.util.Set;

/**
 * SafeFetcher 預設實作，依 40 號設計書 §10.1 12 守則執行。
 *
 * <p>使用 Java 21 {@link HttpClient}（policy=NEVER 自行處理 redirect 以做每跳重驗）。
 * 不依賴外部 HTTP library，避免供應鏈風險。</p>
 *
 * <p>守則 4（DNS rebinding 防護）：每跳先 DNS 預驗確保 hostname 對應的所有 IP 都不在黑名單，
 * 再透過 {@link PinnedInetAddressResolverProvider} 把「已驗 IP」pin 進 thread-scoped 表；
 * HttpClient 連線時內部的第二次 resolve 命中 pin → 直接用已驗 IP，攻擊者在「驗證」與「連線」
 * 之間切換 DNS 也無法把連線導向內網（消滅 TOCTOU 窗口）。pin 在 finally 必清。</p>
 */
public class SafeFetcherImpl implements SafeFetcher {

    private static final Logger LOG = LoggerFactory.getLogger(SafeFetcherImpl.class);

    /** 自家網域 / 內部 hostname 黑名單後綴（小寫比對） */
    private static final List<String> INTERNAL_DOMAIN_SUFFIXES = List.of(
        ".example.com",
        ".internal",
        ".cluster.local",
        ".svc"
    );

    /** 完全比對的內部 hostname */
    private static final Set<String> INTERNAL_HOSTNAMES = Set.of(
        "localhost",
        "example.com"
    );

    private final HttpClient httpClient;

    public SafeFetcherImpl(HttpClient httpClient) {
        this.httpClient = httpClient;
    }

    @Override
    public SafeFetchResponse fetch(SafeFetchRequest request) {
        String currentUrl = request.url();
        int hops = 0;
        while (true) {
            URI uri = parseAndValidateUrl(currentUrl);
            InetAddress[] verifiedIps = validateHostnameAndResolveIps(uri.getHost());

            HttpRequest httpRequest = HttpRequest.newBuilder()
                .uri(uri)
                .timeout(request.readTimeout())
                .header("User-Agent", request.userAgent())
                .header("Accept", request.allowedContentTypes() == null || request.allowedContentTypes().isEmpty()
                    ? "*/*"
                    : String.join(",", request.allowedContentTypes()))
                .GET()
                .build();

            HttpResponse<InputStream> resp;
            // 守則 4：pin 已驗 IP，HttpClient 內部第二次 resolve 命中 pin → 消滅 rebinding TOCTOU。
            PinnedInetAddressResolverProvider.pin(uri.getHost(), verifiedIps);
            try {
                resp = httpClient.send(httpRequest, HttpResponse.BodyHandlers.ofInputStream());
            } catch (HttpTimeoutException e) {
                throw new SafeFetchException(SafeFetchException.Reason.TIMEOUT_READ,
                    "Read timeout: " + currentUrl, e);
            } catch (IOException e) {
                String msg = e.getMessage() == null ? "" : e.getMessage().toLowerCase(Locale.ROOT);
                if (msg.contains("certificate") || msg.contains("ssl") || msg.contains("tls")) {
                    throw new SafeFetchException(SafeFetchException.Reason.TLS_VERIFY_FAILED,
                        "TLS verify failed: " + currentUrl, e);
                }
                throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                    "Fetch failed: " + currentUrl + " — " + e.getMessage(), e);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                    "Interrupted: " + currentUrl, e);
            } finally {
                PinnedInetAddressResolverProvider.unpin(uri.getHost());
            }

            int status = resp.statusCode();
            if (isRedirect(status)) {
                hops++;
                if (hops > request.maxRedirectHops()) {
                    throw new SafeFetchException(SafeFetchException.Reason.REDIRECT_LIMIT_EXCEEDED,
                        "Redirect hop limit exceeded (>" + request.maxRedirectHops() + ")");
                }
                String location = resp.headers().firstValue("Location").orElse(null);
                if (location == null || location.isBlank()) {
                    throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                        "Redirect without Location header: " + currentUrl);
                }
                String resolvedLocation = resolveRelativeLocation(currentUrl, location);
                // 守則 5：禁跨域 redirect
                URI locUri = parseAndValidateUrl(resolvedLocation);
                if (!locUri.getHost().equalsIgnoreCase(uri.getHost())) {
                    throw new SafeFetchException(SafeFetchException.Reason.REDIRECT_CROSS_DOMAIN,
                        "Cross-domain redirect blocked: " + uri.getHost() + " -> " + locUri.getHost());
                }
                currentUrl = resolvedLocation;
                continue;
            }

            if (status == 404 || status == 410) {
                throw new SafeFetchException(SafeFetchException.Reason.NOT_FOUND,
                    "HTTP " + status + ": " + currentUrl);
            }
            if (status >= 400) {
                throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                    "HTTP " + status + ": " + currentUrl);
            }

            // 守則 11：Content-Type 白名單
            String contentType = resp.headers().firstValue("Content-Type").orElse("application/octet-stream");
            String normalizedCt = normalizeContentType(contentType);
            if (request.allowedContentTypes() != null && !request.allowedContentTypes().isEmpty()
                && !request.allowedContentTypes().contains(normalizedCt)) {
                throw new SafeFetchException(SafeFetchException.Reason.CONTENT_TYPE_NOT_ALLOWED,
                    "Content-Type not allowed: " + normalizedCt);
            }

            // 守則 6：Content-Length 預檢 + 邊讀邊計
            resp.headers().firstValueAsLong("Content-Length").ifPresent(cl -> {
                if (cl > request.maxSizeBytes()) {
                    throw new SafeFetchException(SafeFetchException.Reason.SIZE_EXCEEDED,
                        "Content-Length exceeds limit: " + cl + " > " + request.maxSizeBytes());
                }
            });

            byte[] body = readWithSizeGuard(resp.body(), request.maxSizeBytes());
            return new SafeFetchResponse(body, normalizedCt, body.length, currentUrl);
        }
    }

    // ========== 可見供測試用 ==========

    /** 守則 1：協定白名單 — 只允 https。回傳 parsed URI。 */
    URI parseAndValidateUrl(String url) {
        if (url == null || url.isBlank()) {
            throw new SafeFetchException(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED,
                "URL is blank");
        }
        String lower = url.toLowerCase(Locale.ROOT).trim();
        // 早抓 file:// / javascript: 等 — 後面 URI parse 可能容忍
        if (!lower.startsWith("https://")) {
            throw new SafeFetchException(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED,
                "Only https:// is allowed, got: " + safeShortUrl(url));
        }
        URI uri;
        try {
            uri = URI.create(url);
        } catch (IllegalArgumentException e) {
            throw new SafeFetchException(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED,
                "Invalid URI: " + safeShortUrl(url), e);
        }
        if (uri.getScheme() == null || !"https".equalsIgnoreCase(uri.getScheme())) {
            throw new SafeFetchException(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED,
                "Scheme not allowed: " + uri.getScheme());
        }
        if (uri.getHost() == null || uri.getHost().isBlank()) {
            throw new SafeFetchException(SafeFetchException.Reason.PROTOCOL_NOT_ALLOWED,
                "Hostname is missing");
        }
        return uri;
    }

    /**
     * 守則 2/3：hostname 黑名單 + DNS resolve 後 IP 黑名單。
     * 回傳「已通過驗證的 IP[]」供 {@link PinnedInetAddressResolverProvider} pin，確保後續連線走同一批 IP。
     */
    InetAddress[] validateHostnameAndResolveIps(String hostname) {
        String hostLower = hostname.toLowerCase(Locale.ROOT);

        // 自家網域 / 內部 hostname 黑名單
        if (INTERNAL_HOSTNAMES.contains(hostLower)) {
            throw new SafeFetchException(SafeFetchException.Reason.DOMAIN_BLOCKED_INTERNAL,
                "Hostname blocked (internal): " + hostname);
        }
        for (String suffix : INTERNAL_DOMAIN_SUFFIXES) {
            if (hostLower.endsWith(suffix)) {
                throw new SafeFetchException(SafeFetchException.Reason.DOMAIN_BLOCKED_INTERNAL,
                    "Hostname blocked (internal domain): " + hostname);
            }
        }

        // 純 IP literal hostname：直接驗
        if (isIpLiteral(hostname)) {
            InetAddress addr;
            try {
                addr = InetAddress.getByName(hostname);
            } catch (UnknownHostException e) {
                throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                    "Invalid IP literal: " + hostname, e);
            }
            checkIp(addr);
            return new InetAddress[]{addr};
        }

        // DNS resolve（所有 A/AAAA 記錄都要過）
        InetAddress[] addrs;
        try {
            addrs = InetAddress.getAllByName(hostname);
        } catch (UnknownHostException e) {
            throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                "DNS resolve failed: " + hostname, e);
        }
        for (InetAddress addr : addrs) {
            checkIp(addr);
        }
        return addrs;
    }

    /** 守則 2：檢查單個 IP 是否落入黑名單 */
    void checkIp(InetAddress addr) {
        // IPv6 對 IPv4 的封裝先還原成 IPv4 再檢查，避免用 IPv6 形式繞過 IPv4 黑名單
        if (addr instanceof Inet6Address ip6) {
            byte[] b = ip6.getAddress();
            // IPv4-mapped ::ffff:0:0/96（Java 多半已還原成 Inet4Address，此為防禦保險）
            if (isIpv4Mapped(b)) {
                checkIp(ipv4FromTail(b));
                return;
            }
            // NAT64 well-known prefix 64:ff9b::/96 內嵌 IPv4
            if (isNat64(b)) {
                checkIp(ipv4FromTail(b));
                return;
            }
        }
        if (addr.isAnyLocalAddress()) {
            throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_LOOPBACK,
                "IP blocked (any-local): " + addr.getHostAddress());
        }
        if (addr.isLoopbackAddress()) {
            throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_LOOPBACK,
                "IP blocked (loopback): " + addr.getHostAddress());
        }
        if (addr.isLinkLocalAddress()) {
            throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_LINK_LOCAL,
                "IP blocked (link-local): " + addr.getHostAddress());
        }
        if (addr.isSiteLocalAddress()) {
            // 涵蓋 10/8、172.16/12、192.168/16
            throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_PRIVATE,
                "IP blocked (site-local / private): " + addr.getHostAddress());
        }
        if (addr.isMulticastAddress()) {
            throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_MULTICAST,
                "IP blocked (multicast): " + addr.getHostAddress());
        }
        // IPv6 ULA: fc00::/7（Java isSiteLocalAddress 對 IPv6 不涵蓋 fc00::/7，要手動）
        if (addr instanceof Inet6Address ip6) {
            byte first = ip6.getAddress()[0];
            if ((first & 0xfe) == 0xfc) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_PRIVATE,
                    "IP blocked (IPv6 ULA): " + addr.getHostAddress());
            }
        }
        // IPv4 169.254.x.x（Java 對 169.254.x.x 已歸入 link-local，雙重保險）
        if (addr instanceof Inet4Address ip4) {
            byte[] octets = ip4.getAddress();
            int b0 = octets[0] & 0xff;
            int b1 = octets[1] & 0xff;
            if (b0 == 169 && b1 == 254) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_LINK_LOCAL,
                    "IP blocked (link-local): " + addr.getHostAddress());
            }
            // 額外保險：0.0.0.0/8 reserved
            if (b0 == 0) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_LOOPBACK,
                    "IP blocked (0.0.0.0/8 reserved): " + addr.getHostAddress());
            }
            // 100.64.0.0/10 CGNAT 共享位址空間（含阿里雲 metadata 100.100.100.200）
            if (b0 == 100 && b1 >= 64 && b1 <= 127) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_PRIVATE,
                    "IP blocked (CGNAT 100.64.0.0/10): " + addr.getHostAddress());
            }
            // 255.255.255.255 limited broadcast
            int b2 = octets[2] & 0xff;
            int b3 = octets[3] & 0xff;
            if (b0 == 255 && b1 == 255 && b2 == 255 && b3 == 255) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_PRIVATE,
                    "IP blocked (broadcast 255.255.255.255): " + addr.getHostAddress());
            }
            // 240.0.0.0/4 reserved（class E，含 255.0.0.0/8）
            if ((b0 & 0xf0) == 0xf0) {
                throw new SafeFetchException(SafeFetchException.Reason.IP_BLOCKED_PRIVATE,
                    "IP blocked (reserved 240.0.0.0/4): " + addr.getHostAddress());
            }
        }
    }

    /** IPv4-mapped IPv6：::ffff:0:0/96（前 80 bit 為 0，第 11、12 byte 為 0xff）。 */
    private static boolean isIpv4Mapped(byte[] b) {
        for (int i = 0; i < 10; i++) {
            if (b[i] != 0) {
                return false;
            }
        }
        return (b[10] & 0xff) == 0xff && (b[11] & 0xff) == 0xff;
    }

    /** NAT64 well-known prefix 64:ff9b::/96（0064:ff9b 開頭，第 5~12 byte 為 0）。 */
    private static boolean isNat64(byte[] b) {
        if ((b[0] & 0xff) != 0x00 || (b[1] & 0xff) != 0x64
            || (b[2] & 0xff) != 0xff || (b[3] & 0xff) != 0x9b) {
            return false;
        }
        for (int i = 4; i < 12; i++) {
            if (b[i] != 0) {
                return false;
            }
        }
        return true;
    }

    /** 取 16-byte IPv6 位址尾端 4 byte 組出對應的 Inet4Address（getByAddress 不查 DNS）。 */
    private static InetAddress ipv4FromTail(byte[] b) {
        byte[] v4 = new byte[]{b[12], b[13], b[14], b[15]};
        try {
            return InetAddress.getByAddress(v4);
        } catch (UnknownHostException e) {
            // 4-byte 陣列不可能拋此例外
            throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                "Invalid embedded IPv4", e);
        }
    }

    static boolean isIpLiteral(String hostname) {
        // 簡易判斷：含 ':' = IPv6；純數字+'.' = IPv4
        if (hostname.contains(":")) {
            return true;
        }
        String[] parts = hostname.split("\\.");
        if (parts.length != 4) {
            return false;
        }
        for (String p : parts) {
            if (p.isEmpty()) {
                return false;
            }
            for (int i = 0; i < p.length(); i++) {
                if (!Character.isDigit(p.charAt(i))) {
                    return false;
                }
            }
        }
        return true;
    }

    static String normalizeContentType(String raw) {
        if (raw == null) {
            return "";
        }
        int semi = raw.indexOf(';');
        String t = (semi < 0 ? raw : raw.substring(0, semi)).trim().toLowerCase(Locale.ROOT);
        return t;
    }

    static boolean isRedirect(int status) {
        return status == 301 || status == 302 || status == 303 || status == 307 || status == 308;
    }

    static String resolveRelativeLocation(String currentUrl, String location) {
        try {
            URI base = new URI(currentUrl);
            URI rel = new URI(location);
            return base.resolve(rel).toString();
        } catch (URISyntaxException e) {
            return location;
        }
    }

    private byte[] readWithSizeGuard(InputStream in, long maxBytes) {
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        byte[] buf = new byte[8192];
        long total = 0;
        try (in) {
            int n;
            while ((n = in.read(buf)) > 0) {
                total += n;
                if (total > maxBytes) {
                    throw new SafeFetchException(SafeFetchException.Reason.SIZE_EXCEEDED,
                        "Streamed body exceeds limit: " + total + " > " + maxBytes);
                }
                bos.write(buf, 0, n);
            }
        } catch (IOException e) {
            throw new SafeFetchException(SafeFetchException.Reason.FETCH_ERROR,
                "Read body failed: " + e.getMessage(), e);
        }
        return bos.toByteArray();
    }

    private static String safeShortUrl(String url) {
        if (url == null) {
            return "<null>";
        }
        return url.length() > 200 ? url.substring(0, 200) + "..." : url;
    }
}
