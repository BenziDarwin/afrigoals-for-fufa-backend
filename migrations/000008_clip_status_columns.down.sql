DROP INDEX IF EXISTS public.idx_clips_match_event;
DROP INDEX IF EXISTS public.idx_clips_status;
DROP INDEX IF EXISTS public.idx_clips_object_key;

ALTER TABLE public.clips
    DROP COLUMN IF EXISTS object_key,
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS status;
