-- 000004_add_role_check.down.sql
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_role;
