-- Drop index "buyers_current" from table: "buyers"
DROP INDEX "buyer"."buyers_current";
-- Create index "buyers_account" to table: "buyers"
CREATE UNIQUE INDEX "buyers_account" ON "buyer"."buyers" ("account_id");
