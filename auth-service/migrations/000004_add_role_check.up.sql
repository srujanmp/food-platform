-- 000004_add_role_check.up.sql
ALTER TABLE users ADD CONSTRAINT chk_role CHECK (role IN ('USER', 'RESTAURANT_OWNER', 'DRIVER', 'ADMIN'));
