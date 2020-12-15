-- This script only contains the table creation statements and does not fully represent the table in database. It's still missing: indices, triggers. Do not use it as backup.

-- Table Definition
CREATE TABLE "public"."vouchers" (
    "id" uuid,
    "quota" numeric
);

INSERT INTO "public"."vouchers" ("id", "quota") VALUES
('acfaa261-82f7-4132-9687-b44a203c5951', 100);
INSERT INTO "public"."vouchers" ("id", "quota") VALUES
('7671068b-e450-4aa9-b7a3-cd100203a3c9', 100);
