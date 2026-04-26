-- Drops the app_auth schema and its tables. Postgres can't remove enum
-- values without recreating the type; LOCAL_GOOGLE and LOCAL_SMS stay in
-- the enum after a rollback, which is harmless (no rows reference them
-- once user_session is gone).
DROP SCHEMA IF EXISTS app_auth CASCADE;
