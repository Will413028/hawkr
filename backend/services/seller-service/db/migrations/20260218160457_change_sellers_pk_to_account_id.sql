-- Modify "sellers" table
ALTER TABLE "sellers" DROP CONSTRAINT "sellers_pkey", DROP CONSTRAINT "sellers_account_id_key", DROP COLUMN "id", ALTER COLUMN "approval_status" SET DEFAULT 'unregistered', DROP COLUMN "payuni_mer_id", ADD PRIMARY KEY ("account_id");
