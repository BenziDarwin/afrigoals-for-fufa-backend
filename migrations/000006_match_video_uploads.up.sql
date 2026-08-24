CREATE TABLE IF NOT EXISTS public.match_video_uploads (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    match_id bigint NOT NULL,
    user_id bigint NOT NULL,
    file_name varchar(255) NOT NULL,
    storage_key varchar(500) NOT NULL,
    upload_id varchar(500) NOT NULL,
    file_size bigint NOT NULL,
    chunk_size bigint NOT NULL,
    total_parts integer NOT NULL,
    uploaded_parts jsonb NOT NULL DEFAULT '{}'::jsonb,
    status varchar(32) NOT NULL DEFAULT 'initiated',
    CONSTRAINT match_video_uploads_status_check
        CHECK (status IN ('initiated', 'uploading', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_match_video_uploads_match_id
    ON public.match_video_uploads USING btree (match_id);

CREATE INDEX IF NOT EXISTS idx_match_video_uploads_user_id
    ON public.match_video_uploads USING btree (user_id);

CREATE INDEX IF NOT EXISTS idx_match_video_uploads_storage_key
    ON public.match_video_uploads USING btree (storage_key);

CREATE INDEX IF NOT EXISTS idx_match_video_uploads_upload_id
    ON public.match_video_uploads USING btree (upload_id);

CREATE INDEX IF NOT EXISTS idx_match_video_uploads_status
    ON public.match_video_uploads USING btree (status);
