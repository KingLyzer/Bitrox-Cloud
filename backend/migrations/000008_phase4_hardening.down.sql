DROP TABLE IF EXISTS share_access_grants;
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS calendar_event_reminders;
DROP TABLE IF EXISTS calendar_events;
DELETE FROM app_settings WHERE key IN ('general', 'sharing', 'security', 'other');
DROP TABLE IF EXISTS app_settings;