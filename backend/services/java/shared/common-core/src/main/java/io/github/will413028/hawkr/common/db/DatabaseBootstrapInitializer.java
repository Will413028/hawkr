package io.github.will413028.hawkr.common.db;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.regex.Pattern;

import org.springframework.beans.BeansException;
import org.springframework.beans.factory.config.BeanFactoryPostProcessor;
import org.springframework.beans.factory.config.ConfigurableListableBeanFactory;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.core.env.Environment;

import lombok.extern.slf4j.Slf4j;

/**
 * 在 Spring DataSource 初始化之前，確保目標 DB 與必要 extension 存在。
 * <p>
 * 透過 BeanFactoryPostProcessor 介面，於所有業務 bean（含 DataSource）建立之前執行。
 * 從 Environment 直接綁定 DatabaseBootstrapProperties，避免依賴尚未建立的 DataSource bean。
 * <p>
 * 詳見 38-Flyway 自動同步與 DB 自動初始化詳細設計書 §3.4。
 */
@Slf4j
public class DatabaseBootstrapInitializer implements BeanFactoryPostProcessor {

    /**
     * 合法 DB / extension 名稱：英數 + 底線，避免 SQL injection。
     */
    private static final Pattern SAFE_IDENT = Pattern.compile("^[a-zA-Z_][a-zA-Z0-9_]*$");

    private final Environment environment;

    public DatabaseBootstrapInitializer(Environment environment) {
        this.environment = environment;
    }

    @Override
    public void postProcessBeanFactory(ConfigurableListableBeanFactory beanFactory) throws BeansException {
        DatabaseBootstrapProperties props = Binder.get(environment)
                .bind("hawkr.db.bootstrap", DatabaseBootstrapProperties.class)
                .orElseGet(DatabaseBootstrapProperties::new);

        if (!props.isEnabled()) {
            log.info("DatabaseBootstrap.skipped reason=disabled");
            return;
        }

        validateProps(props);

        ensureDatabase(props);
        ensureExtensions(props);

        log.info("DatabaseBootstrap.completed target={}", props.getTargetDatabase());
    }

    private void validateProps(DatabaseBootstrapProperties p) {
        if (isBlank(p.getAdminUrl())) {
            throw new IllegalStateException("hawkr.db.bootstrap.admin-url is required when enabled=true");
        }
        if (isBlank(p.getAdminUser())) {
            throw new IllegalStateException("hawkr.db.bootstrap.admin-user is required when enabled=true");
        }
        if (isBlank(p.getAdminPassword())) {
            throw new IllegalStateException("hawkr.db.bootstrap.admin-password is required when enabled=true");
        }
        if (isBlank(p.getTargetDatabase())) {
            throw new IllegalStateException("hawkr.db.bootstrap.target-database is required when enabled=true");
        }
        if (!SAFE_IDENT.matcher(p.getTargetDatabase()).matches()) {
            throw new IllegalStateException("Invalid target database name: " + p.getTargetDatabase());
        }
        for (DatabaseBootstrapProperties.Extension ext : p.getExtensions()) {
            if (ext.isEnabled() && (ext.getName() == null || !SAFE_IDENT.matcher(ext.getName()).matches())) {
                throw new IllegalStateException("Invalid extension name: " + ext.getName());
            }
        }
    }

    private void ensureDatabase(DatabaseBootstrapProperties p) {
        try (Connection conn = DriverManager.getConnection(p.getAdminUrl(), p.getAdminUser(), p.getAdminPassword())) {
            try (PreparedStatement ps = conn.prepareStatement("SELECT 1 FROM pg_database WHERE datname = ?")) {
                ps.setString(1, p.getTargetDatabase());
                try (ResultSet rs = ps.executeQuery()) {
                    if (rs.next()) {
                        log.info("DatabaseBootstrap.createDatabase.skipped db={} reason=already_exists",
                                p.getTargetDatabase());
                        return;
                    }
                }
            }

            try (Statement st = conn.createStatement()) {
                // DB 名稱已經過 SAFE_IDENT 驗證，可以安全拼接
                st.executeUpdate("CREATE DATABASE " + p.getTargetDatabase());
                log.info("DatabaseBootstrap.createDatabase.success db={}", p.getTargetDatabase());
            }
        } catch (SQLException e) {
            throw new IllegalStateException(
                    "DatabaseBootstrap.createDatabase.failed db=" + p.getTargetDatabase() + ": " + e.getMessage(), e);
        }
    }

    private void ensureExtensions(DatabaseBootstrapProperties p) {
        long enabledCount = p.getExtensions().stream().filter(DatabaseBootstrapProperties.Extension::isEnabled).count();
        if (enabledCount == 0) {
            return;
        }

        String targetUrl = swapDatabase(p.getAdminUrl(), p.getTargetDatabase());

        try (Connection conn = DriverManager.getConnection(targetUrl, p.getAdminUser(), p.getAdminPassword());
             Statement st = conn.createStatement()) {
            for (DatabaseBootstrapProperties.Extension ext : p.getExtensions()) {
                if (!ext.isEnabled()) {
                    continue;
                }
                // Extension 名稱已通過 SAFE_IDENT 驗證
                st.executeUpdate("CREATE EXTENSION IF NOT EXISTS " + ext.getName());
                log.info("DatabaseBootstrap.createExtension.success db={} extension={}",
                        p.getTargetDatabase(), ext.getName());
            }
        } catch (SQLException e) {
            throw new IllegalStateException(
                    "DatabaseBootstrap.createExtension.failed db=" + p.getTargetDatabase() + ": " + e.getMessage(), e);
        }
    }

    /**
     * 將 admin-url（指向 postgres 系統 DB）改為指向 target DB。
     * 範例：jdbc:postgresql://host:5432/postgres → jdbc:postgresql://host:5432/hawkr_product
     */
    private String swapDatabase(String adminUrl, String targetDb) {
        int lastSlash = adminUrl.lastIndexOf('/');
        if (lastSlash < 0) {
            throw new IllegalStateException("Invalid admin-url: " + adminUrl);
        }
        int questionMark = adminUrl.indexOf('?', lastSlash);
        String prefix = adminUrl.substring(0, lastSlash + 1);
        String suffix = questionMark >= 0 ? adminUrl.substring(questionMark) : "";
        return prefix + targetDb + suffix;
    }

    private boolean isBlank(String s) {
        return s == null || s.trim().isEmpty();
    }
}
