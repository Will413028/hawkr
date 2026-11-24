-- Create index "campaigns_admin_list" to table: "coupon_campaigns"
CREATE INDEX "campaigns_admin_list" ON "coupon"."coupon_campaigns" ("created_at" DESC, "id" DESC) WHERE (target_type <> 'COMPENSATION'::text);
