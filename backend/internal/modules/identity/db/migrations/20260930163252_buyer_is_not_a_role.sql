-- 買家不再存成角色（Will 2026-10-01 W5）：帳號活著就是買家。先刪掉既有的 BUYER 列，再收緊 CHECK。
DELETE FROM "identity"."account_roles" WHERE "role" = 'BUYER';
-- Modify "account_roles" table
ALTER TABLE "identity"."account_roles" DROP CONSTRAINT "account_roles_role_check", ADD CONSTRAINT "account_roles_role_check" CHECK (role = 'SELLER'::text);
