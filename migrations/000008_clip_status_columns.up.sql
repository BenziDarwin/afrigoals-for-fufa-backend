ALTER TABLE public.clips
    ADD COLUMN IF NOT EXISTS status character varying(20) NOT NULL DEFAULT 'processing',
    ADD COLUMN IF NOT EXISTS error_message character varying(500),
    ADD COLUMN IF NOT EXISTS object_key character varying(500);

UPDATE public.clips
SET status = 'completed'
WHERE clip_url IS NOT NULL
    AND btrim(clip_url) <> ''
    AND clip_url NOT LIKE '/uploads/clips/%';

UPDATE public.clips
SET status = 'pending', clip_url = NULL, object_key = NULL
WHERE clip_url LIKE '/uploads/clips/%';

UPDATE public.clips
SET status = 'processing'
WHERE status IS NULL OR status = '';

CREATE INDEX IF NOT EXISTS idx_clips_status
    ON public.clips USING btree (status);

CREATE INDEX IF NOT EXISTS idx_clips_match_event
    ON public.clips USING btree (match_id, event_id);

CREATE INDEX IF NOT EXISTS idx_clips_object_key
    ON public.clips USING btree (object_key);
