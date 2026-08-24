ALTER TABLE public.match_videos
    ADD COLUMN IF NOT EXISTS duration_sec bigint,
    ADD COLUMN IF NOT EXISTS uploaded_by bigint;

CREATE INDEX IF NOT EXISTS idx_match_videos_uploaded_by
    ON public.match_videos USING btree (uploaded_by);
