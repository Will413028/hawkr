-- Modify "certification_applications" table
-- #608: id_card_no stored as AES-256-GCM Base64 ciphertext. Widen varchar(20)
-- -> varchar(255) to hold the ~52-char Base64 of a ~10-char id's ciphertext.
-- Column name unchanged (semantics documented at the application layer).
ALTER TABLE "certification_applications" ALTER COLUMN "id_card_no" TYPE character varying(255);
