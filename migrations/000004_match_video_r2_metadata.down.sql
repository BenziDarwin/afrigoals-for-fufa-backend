DROP INDEX IF EXISTS public.idx_match_videos_uploaded_by;

ALTER TABLE public.match_videos
    DROP COLUMN IF EXISTS uploaded_by,
    DROP COLUMN IF EXISTS duration_sec;
