package io.github.will413028.hawkr.common.obs.config;

/**
 * Configuration for a gRPC target service.
 */
public class GrpcTarget {

    private String host;
    private int port;
    private boolean plaintext;

    public String getHost() {
        return host;
    }

    public void setHost(String host) {
        this.host = host;
    }

    public int getPort() {
        return port;
    }

    public void setPort(int port) {
        this.port = port;
    }

    public boolean isPlaintext() {
        return plaintext;
    }

    public void setPlaintext(boolean plaintext) {
        this.plaintext = plaintext;
    }
}
