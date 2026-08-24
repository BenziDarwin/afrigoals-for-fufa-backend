ALTER TABLE public.analysis_events
    ALTER COLUMN id DROP DEFAULT;

DROP SEQUENCE IF EXISTS public.analysis_events_id_seq;
