-- Modify "product_views_daily" table
ALTER TABLE "seller"."product_views_daily" ADD CONSTRAINT "product_views_daily_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
