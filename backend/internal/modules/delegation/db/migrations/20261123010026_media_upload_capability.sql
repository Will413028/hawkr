-- Modify "admin_operation_audit_logs" table
ALTER TABLE "delegation"."admin_operation_audit_logs" DROP CONSTRAINT "admin_operation_audit_logs_action_check", ADD CONSTRAINT "admin_operation_audit_logs_action_check" CHECK (action = ANY (ARRAY['STOREFRONT_UPDATE'::text, 'STOREFRONT_LOGO_UPDATE'::text, 'STOREFRONT_COVER_UPDATE'::text, 'PRODUCT_CREATE'::text, 'PRODUCT_UPDATE'::text, 'PRODUCT_PUBLISH'::text, 'PRODUCT_UNPUBLISH'::text, 'PRODUCT_BATCH_UNPUBLISH'::text, 'POST_CREATE'::text, 'POST_UPDATE'::text, 'POST_PUBLISH'::text, 'POST_UNPUBLISH'::text, 'POST_SCHEDULE'::text, 'ORDER_PII_REVEAL'::text, 'STOREFRONT_READ'::text, 'PRODUCT_READ'::text, 'POST_READ'::text, 'ORDER_READ'::text, 'AUDIT_CORRECTION'::text, 'MEDIA_UPLOAD'::text])), DROP CONSTRAINT "admin_operation_audit_logs_capability_check", ADD CONSTRAINT "admin_operation_audit_logs_capability_check" CHECK (capability = ANY (ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text, 'media.upload'::text])), DROP CONSTRAINT "admin_operation_audit_logs_resource_type_check", ADD CONSTRAINT "admin_operation_audit_logs_resource_type_check" CHECK (resource_type = ANY (ARRAY['STOREFRONT'::text, 'PRODUCT'::text, 'POST'::text, 'ORDER'::text, 'MEDIA'::text]));
-- Modify "delegated_operation_sessions" table
ALTER TABLE "delegation"."delegated_operation_sessions" DROP CONSTRAINT "delegated_operation_sessions_capabilities_check", ADD CONSTRAINT "delegated_operation_sessions_capabilities_check" CHECK ((cardinality(capabilities) >= 1) AND (cardinality(capabilities) <= 10) AND (capabilities <@ ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text, 'media.upload'::text]) AND (cardinality(capabilities) = (((((((((
CASE
    WHEN ('storefront.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END +
CASE
    WHEN ('storefront.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.publish'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.publish'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('orders.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('media.upload'::text = ANY (capabilities)) THEN 1
    ELSE 0
END)));
-- Modify "seller_delegation_authorizations" table
ALTER TABLE "delegation"."seller_delegation_authorizations" DROP CONSTRAINT "seller_delegation_authorizations_capabilities_check", ADD CONSTRAINT "seller_delegation_authorizations_capabilities_check" CHECK ((cardinality(granted_capabilities) >= 1) AND (cardinality(granted_capabilities) <= 10) AND (granted_capabilities <@ ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text, 'media.upload'::text]) AND (cardinality(granted_capabilities) = (((((((((
CASE
    WHEN ('storefront.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END +
CASE
    WHEN ('storefront.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.publish'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.publish'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('orders.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('media.upload'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END)));
