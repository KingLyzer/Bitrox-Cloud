CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value_json JSONB NOT NULL,
    updated_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_settings (key, value_json)
VALUES
    (
        'general',
        jsonb_build_object(
            'site_name', 'BitroxCloud',
            'site_logo_url', 'https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png',
            'favicon_url', '',
            'brand_color', '#2563eb',
            'default_storage_quota_bytes', 21474836480,
            'default_language', 'tr',
            'default_timezone', 'Europe/Istanbul',
            'public_base_url', '',
            'maintenance_mode', false
        )
    ),
    (
        'sharing',
        jsonb_build_object(
            'public_sharing_enabled', true,
            'allow_password_protected_links', true,
            'allow_expiration', true,
            'default_expiration_days', 7,
            'maximum_expiration_days', 90,
            'allow_public_downloads', true,
            'allow_folder_sharing', false,
            'require_password_for_public_links', false
        )
    ),
    (
        'security',
        jsonb_build_object(
            'minimum_password_length', 12,
            'require_uppercase', true,
            'require_lowercase', true,
            'require_number', true,
            'require_symbol', false,
            'session_lifetime_minutes', 15,
            'refresh_token_lifetime_minutes', 43200,
            'two_factor_required', false,
            'login_rate_limit_per_window', 10,
            'login_rate_window_seconds', 60,
            'trusted_proxy_note', 'APP_TRUSTED_PROXY_CIDRS ve APP_COOKIE_SECURE ayarlarina dikkat edin.'
        )
    ),
    (
        'other',
        jsonb_build_object(
            'storage_cleanup_interval_seconds', 60,
            'upload_max_file_size_bytes', 107374182400,
            'upload_max_chunk_size_bytes', 16777216,
            'preview_generation_enabled', false,
            'logging_level', 'info',
            'requires_restart_note', 'Bazi ayarlar runtime yerine env tabanlidir; degisiklikler icin restart gerekir.'
        )
    )
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS calendar_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    location TEXT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT calendar_events_time_check CHECK (end_at > start_at)
);

CREATE INDEX IF NOT EXISTS calendar_events_user_start_idx
    ON calendar_events (user_id, start_at)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS calendar_event_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reminder_type TEXT NOT NULL CHECK (reminder_type IN ('at_time', '10m', '1h', '1d', '3d')),
    remind_at TIMESTAMPTZ NOT NULL,
    delivered_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, reminder_type)
);

CREATE INDEX IF NOT EXISTS calendar_event_reminders_due_idx
    ON calendar_event_reminders (remind_at)
    WHERE delivered_at IS NULL;

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMPTZ NULL,
    source_reminder_id UUID NULL UNIQUE REFERENCES calendar_event_reminders(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_user_created_idx ON notifications (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS notifications_user_unread_idx ON notifications (user_id, is_read);

CREATE TABLE IF NOT EXISTS shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    password_hash TEXT NULL,
    expires_at TIMESTAMPTZ NULL,
    max_downloads INTEGER NULL CHECK (max_downloads IS NULL OR max_downloads > 0),
    download_count INTEGER NOT NULL DEFAULT 0 CHECK (download_count >= 0),
    allow_download BOOLEAN NOT NULL DEFAULT true,
    allow_preview BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS shares_owner_node_idx ON shares (owner_user_id, node_id);
CREATE INDEX IF NOT EXISTS shares_active_idx ON shares (token_hash) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS share_access_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    share_id UUID NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    grant_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS share_access_grants_share_idx ON share_access_grants (share_id);
