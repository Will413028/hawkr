package io.github.will413028.hawkr.payment.persistence;

import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.sql.Connection;
import java.sql.DatabaseMetaData;
import java.sql.DriverManager;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.Set;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Shared billing schema contract. The payment database owns provider facts;
 * buyer/seller membership tables must never be introduced here.
 */
@Testcontainers
@Tag("integration")
class MembershipBillingMigrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("membership_billing_migration_test")
            .withUsername("test")
            .withPassword("test")
            .withEnv("TZ", "UTC")
            .withEnv("PGTZ", "UTC");

    @BeforeAll
    static void migrate() {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration")
                .baselineOnMigrate(false)
                .load()
                .migrate();
    }

    @AfterAll
    static void close() {
        POSTGRES.stop();
    }

    @Test
    void createsSharedBillingTablesAlongsideLegacyPaymentTables() throws SQLException {
        try (Connection connection = connection()) {
            DatabaseMetaData metadata = connection.getMetaData();
            for (String table : Set.of(
                    "payment_transactions", "payment_refunds", "outbox_events",
                    "billing_provider_accounts", "membership_provider_subscriptions",
                    "membership_charge_attempts", "billing_commands")) {
                try (ResultSet tables = metadata.getTables(null, "public", table, new String[]{"TABLE"})) {
                    assertTrue(tables.next(), "missing table " + table);
                }
            }
            assertFalse(tableExists(metadata, "buyer_memberships"),
                    "payment DB must not own buyer membership tables");
            assertFalse(tableExists(metadata, "seller_memberships"),
                    "payment DB must not own seller membership tables");
        }
    }

    @Test
    void usesAuditSafeTypesAndConstraints() throws SQLException {
        try (Connection connection = connection(); Statement statement = connection.createStatement()) {
            assertEquals("timestamp with time zone", columnType(statement,
                    "membership_provider_subscriptions", "quote_expires_at"));
            assertEquals("bigint", columnType(statement,
                    "membership_provider_subscriptions", "unit_amount_minor"));
            assertEquals("timestamp with time zone", columnType(statement,
                    "membership_charge_attempts", "valid_from"));
            assertEquals("timestamp with time zone", columnType(statement,
                    "membership_charge_attempts", "cycle_started_at"));
            assertEquals("boolean", columnType(statement,
                    "membership_charge_attempts", "is_initial_authorization"));
            assertEquals("character", columnType(statement,
                    "membership_charge_attempts", "quote_snapshot_hash"));
            assertEquals("text", columnType(statement, "billing_commands", "error_message"));
            assertEquals("text", columnType(statement, "billing_commands", "result_redirect_url"));
            assertEquals("text", columnType(statement, "billing_commands", "action_reason"));
            assertEquals("character varying", columnType(statement,
                    "billing_commands", "result_provider_subscription_id"));
            assertEquals("timestamp with time zone", columnType(statement,
                    "billing_commands", "lease_expires_at"));

            assertTrue(indexExists(statement, "uq_membership_provider_subscriptions_active_base"),
                    "one active BASE_MEMBERSHIP subscription per subject must be enforced");
            assertTrue(indexExists(statement, "uq_membership_provider_subscriptions_provider_id"),
                    "provider subscription identity must be unique when present");
            assertTrue(indexExists(statement, "uq_membership_provider_subscriptions_subject_sequence"),
                    "subscription sequence must be unique per subject");
            assertTrue(indexExists(statement, "uq_membership_charge_attempts_provider_event_key"),
                    "provider notification dedup key must be unique");
            assertTrue(indexExists(statement, "uq_membership_charge_attempts_authoritative_period"),
                    "first terminal result per provider period must be unique");
            assertTrue(indexExists(statement, "uq_billing_commands_operation_key"),
                    "command idempotency key must be unique per operation");
            assertTrue(indexExists(statement, "uq_billing_commands_active_subscription"),
                    "a subscription must have at most one in-flight or unknown lifecycle command");
        }
    }

    @Test
    void allowsCommandClaimBeforeSubscriptionWhenTheResultReferenceIsBoundInTheSameTransaction()
            throws SQLException {
        UUID commandId = UUID.randomUUID();
        long providerAccountId = 9_900_000_000_000_099L;
        long subscriptionId = 9_900_000_000_000_199L;
        String accountRef = "migration-command-order-" + commandId;
        String quoteId = "migration-command-order-" + commandId;

        try {
            try (Connection connection = connection()) {
                connection.setAutoCommit(false);
                try {
                    try (var statement = connection.prepareStatement("""
                            INSERT INTO billing_provider_accounts
                                (id, provider, account_ref, environment, credential_ref, capabilities, status)
                            VALUES (?, 'PAYUNI', ?, 'SANDBOX', 'test/disabled', '{}'::jsonb, 'ACTIVE')
                            """)) {
                        statement.setLong(1, providerAccountId);
                        statement.setString(2, accountRef);
                        statement.executeUpdate();
                    }

                    // The idempotency claim intentionally happens before the parent row exists.
                    // result_subscription_id stays NULL until the parent insert below succeeds.
                    try (var statement = connection.prepareStatement("""
                            INSERT INTO billing_commands
                                (id, operation, subject_type, subject_id, idempotency_key, request_hash,
                                 billing_quote_id, result_subscription_id, status)
                            VALUES (?, 'CREATE_SUBSCRIPTION', 'BUYER', 'migration-buyer', ?, ?, ?, NULL, 'PROCESSING')
                            """)) {
                        statement.setObject(1, commandId);
                        statement.setString(2, "request-" + commandId);
                        statement.setString(3, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
                        statement.setString(4, quoteId);
                        statement.executeUpdate();
                    }

                    try (var statement = connection.prepareStatement("""
                            INSERT INTO membership_provider_subscriptions
                                (id, subject_type, subject_id, membership_id, billing_quote_id, subscription_kind,
                                 provider_account_ref, provider, subscription_seq, mer_trade_no, offer_id,
                                 offer_version_no, quote_snapshot_hash, quote_expires_at, currency, unit_amount_minor,
                                 interval_unit, interval_count, status)
                            VALUES (?, 'BUYER', 'migration-buyer', 'migration-membership', ?, 'BASE_MEMBERSHIP',
                                    ?, 'PAYUNI', 1, ?, 'migration-offer', '1',
                                    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
                                    NOW() + INTERVAL '10 minutes', 'TWD', 19900, 'MONTH', 1, 'PENDING')
                            """)) {
                        statement.setLong(1, subscriptionId);
                        statement.setString(2, quoteId);
                        statement.setString(3, accountRef);
                        statement.setString(4, "MIGRATION-" + commandId);
                        statement.executeUpdate();
                    }

                    try (var statement = connection.prepareStatement("""
                            UPDATE billing_commands
                            SET result_subscription_id = ?, updated_at = NOW()
                            WHERE id = ? AND status = 'PROCESSING' AND result_subscription_id IS NULL
                            """)) {
                        statement.setLong(1, subscriptionId);
                        statement.setObject(2, commandId);
                        assertEquals(1, statement.executeUpdate());
                    }
                    connection.commit();

                    try (var statement = connection.prepareStatement(
                            "SELECT result_subscription_id FROM billing_commands WHERE id = ?")) {
                        statement.setObject(1, commandId);
                        try (ResultSet result = statement.executeQuery()) {
                            assertTrue(result.next());
                            assertEquals(subscriptionId, result.getLong(1));
                        }
                    }
                } catch (RuntimeException | SQLException failure) {
                    connection.rollback();
                    throw failure;
                } finally {
                    connection.setAutoCommit(true);
                }
            }
        } finally {
            try (Connection cleanup = connection()) {
                try (var command = cleanup.prepareStatement("DELETE FROM billing_commands WHERE id = ?")) {
                    command.setObject(1, commandId);
                    command.executeUpdate();
                }
                try (var subscription = cleanup.prepareStatement(
                        "DELETE FROM membership_provider_subscriptions WHERE id = ?")) {
                    subscription.setLong(1, subscriptionId);
                    subscription.executeUpdate();
                }
                try (var account = cleanup.prepareStatement(
                        "DELETE FROM billing_provider_accounts WHERE id = ?")) {
                    account.setLong(1, providerAccountId);
                    account.executeUpdate();
                }
            }
        }
    }

    private static Connection connection() throws SQLException {
        return DriverManager.getConnection(
                POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
    }

    private static boolean tableExists(DatabaseMetaData metadata, String table) throws SQLException {
        try (ResultSet tables = metadata.getTables(null, "public", table, new String[]{"TABLE"})) {
            return tables.next();
        }
    }

    private static String columnType(Statement statement, String table, String column) throws SQLException {
        String sql = "SELECT data_type FROM information_schema.columns "
                + "WHERE table_schema='public' AND table_name='" + table
                + "' AND column_name='" + column + "'";
        try (ResultSet result = statement.executeQuery(sql)) {
            assertTrue(result.next(), "missing column " + table + "." + column);
            return result.getString(1);
        }
    }

    private static boolean indexExists(Statement statement, String index) throws SQLException {
        try (ResultSet result = statement.executeQuery(
                "SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='" + index + "'")) {
            return result.next();
        }
    }
}
