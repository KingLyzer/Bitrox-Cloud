UPDATE app_settings
SET value_json = jsonb_set(
    value_json,
    '{site_logo_url}',
    to_jsonb('https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png'::text),
    true
),
updated_at = now()
WHERE key = 'general'
  AND COALESCE(NULLIF(value_json->>'site_logo_url', ''), '') = '';
