ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferred_language TEXT NULL;

UPDATE app_settings
SET value_json = value_json
    || jsonb_build_object(
        'site_subtitle', COALESCE(value_json->>'site_subtitle', 'Private storage'),
        'browser_title', COALESCE(value_json->>'browser_title', COALESCE(value_json->>'site_name', 'BitroxCloud')),
        'default_language', COALESCE(NULLIF(value_json->>'default_language', ''), 'en')
    ),
    updated_at = now()
WHERE key = 'general';
