-- Modify "food_registrations" table
ALTER TABLE "seller"."food_registrations" DROP CONSTRAINT "food_registrations_number_check", ADD CONSTRAINT "food_registrations_number_check" CHECK (((char_length(registration_number) >= 1) AND (char_length(registration_number) <= 50)) AND (registration_number !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text));
-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" DROP CONSTRAINT "kyc_applications_real_name_check", ADD CONSTRAINT "kyc_applications_real_name_check" CHECK ((char_length(real_name) >= 1) AND (char_length(real_name) <= 100) AND (real_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text));
-- Modify "prospect_contacts" table
ALTER TABLE "seller"."prospect_contacts" DROP CONSTRAINT "prospect_contacts_value_check", ADD CONSTRAINT "prospect_contacts_value_check" CHECK ((char_length(value) >= 1) AND (value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text) AND (char_length(value) <=
CASE kind
    WHEN 'EMAIL'::text THEN 255
    WHEN 'PHONE'::text THEN 50
    ELSE 500
END));
-- Drop index "prospects_active_name" from table: "prospects"
DROP INDEX "seller"."prospects_active_name";
-- Modify "prospects" table
ALTER TABLE "seller"."prospects" DROP CONSTRAINT "prospects_display_name_check", ADD CONSTRAINT "prospects_display_name_check" CHECK ((char_length(display_name) >= 1) AND (char_length(display_name) <= 255) AND (display_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text));
-- Create index "prospects_active_name" to table: "prospects"
CREATE INDEX "prospects_active_name" ON "seller"."prospects" ((lower(display_name)), "prospect_id") WHERE (status = 'ACTIVE'::text);
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_contact_name_check", ADD CONSTRAINT "stores_contact_name_check" CHECK (((char_length(contact_name) >= 1) AND (char_length(contact_name) <= 20)) AND (contact_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text));
