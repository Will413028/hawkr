package io.github.will413028.hawkr.common.db;

import java.util.ArrayList;
import java.util.List;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * DB Bootstrap 配置（部署即自動建立 DB 與 extension）
 * <p>
 * 由 DatabaseBootstrapAutoConfiguration 自動載入，於 Spring DataSource 初始化前執行。
 * 詳見 38-Flyway 自動同步與 DB 自動初始化詳細設計書 §3.4。
 */
@ConfigurationProperties(prefix = "hawkr.db.bootstrap")
public class DatabaseBootstrapProperties {

    /**
     * 是否啟用 bootstrap。緊急情況可設 false 跳過。
     */
    private boolean enabled = true;

    /**
     * 連到 postgres 系統 DB 的 JDBC URL（用於 CREATE DATABASE）。
     */
    private String adminUrl;

    /**
     * Bootstrap 階段使用的 admin 帳號（需 CREATEDB 權限，或 superuser）。
     */
    private String adminUser;

    /**
     * Bootstrap 階段使用的 admin 密碼。
     */
    private String adminPassword;

    /**
     * 目標 DB 名稱（會被自動建立，若不存在）。
     */
    private String targetDatabase;

    /**
     * 需要在目標 DB 安裝的 extension 清單。
     */
    private List<Extension> extensions = new ArrayList<>();

    public boolean isEnabled() {
        return enabled;
    }

    public void setEnabled(boolean enabled) {
        this.enabled = enabled;
    }

    public String getAdminUrl() {
        return adminUrl;
    }

    public void setAdminUrl(String adminUrl) {
        this.adminUrl = adminUrl;
    }

    public String getAdminUser() {
        return adminUser;
    }

    public void setAdminUser(String adminUser) {
        this.adminUser = adminUser;
    }

    public String getAdminPassword() {
        return adminPassword;
    }

    public void setAdminPassword(String adminPassword) {
        this.adminPassword = adminPassword;
    }

    public String getTargetDatabase() {
        return targetDatabase;
    }

    public void setTargetDatabase(String targetDatabase) {
        this.targetDatabase = targetDatabase;
    }

    public List<Extension> getExtensions() {
        return extensions;
    }

    public void setExtensions(List<Extension> extensions) {
        this.extensions = extensions;
    }

    public static class Extension {

        /**
         * Extension 名稱（如 pg_trgm）。
         */
        private String name;

        /**
         * 是否啟用此 extension。預設 false，由各服務透過 env var 開啟。
         */
        private boolean enabled = false;

        public String getName() {
            return name;
        }

        public void setName(String name) {
            this.name = name;
        }

        public boolean isEnabled() {
            return enabled;
        }

        public void setEnabled(boolean enabled) {
            this.enabled = enabled;
        }
    }
}
