CREATE INDEX IF NOT EXISTS idx_videos_match_id_id
    ON public.videos USING btree (match_id, id);
