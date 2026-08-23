ALTER TABLE public.analysis_event_stats
    RENAME CONSTRAINT fk_analysis_events_stats TO fk_analysis_event_stats_analysis_event;

ALTER TABLE public.leagues ALTER COLUMN uuid DROP DEFAULT;

DROP TABLE IF EXISTS player_analysis_reports CASCADE;