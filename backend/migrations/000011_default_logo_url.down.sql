UPDATE app_settings
SET value_json = jsonb_set(
    value_json,
    '{site_logo_url}',
    to_jsonb(''::text),
    true
),
updated_at = now()
WHERE key = 'general'
  AND value_json->>'site_logo_url' = 'https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png';
