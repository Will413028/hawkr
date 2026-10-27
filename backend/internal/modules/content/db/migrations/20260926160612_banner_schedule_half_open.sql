-- Modify "banners" table
ALTER TABLE "content"."banners" DROP CONSTRAINT "banners_schedule_check", ADD CONSTRAINT "banners_schedule_check" CHECK ((start_date IS NULL) OR (end_date IS NULL) OR (start_date < end_date));
