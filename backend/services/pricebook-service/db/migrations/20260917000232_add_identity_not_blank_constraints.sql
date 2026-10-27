ALTER TABLE "crop" ADD CONSTRAINT "crop_code_not_blank" CHECK (length(btrim(crop_code)) > 0), ADD CONSTRAINT "crop_name_not_blank" CHECK (length(btrim(name)) > 0);
ALTER TABLE "crop_group" ADD CONSTRAINT "crop_group_code_not_blank" CHECK (length(btrim(group_code)) > 0), ADD CONSTRAINT "crop_group_name_not_blank" CHECK (length(btrim(name)) > 0);
ALTER TABLE "market" ADD CONSTRAINT "market_code_not_blank" CHECK (length(btrim(market_code)) > 0), ADD CONSTRAINT "market_name_not_blank" CHECK (length(btrim(name)) > 0);
