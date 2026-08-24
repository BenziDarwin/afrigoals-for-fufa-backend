DROP INDEX IF EXISTS public.idx_analysis_events_secondary_player_id;
DROP INDEX IF EXISTS public.idx_analysis_events_team_id;
DROP INDEX IF EXISTS public.idx_analysis_events_event_type_id;

ALTER TABLE public.analysis_events
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS outcome,
    DROP COLUMN IF EXISTS pitch_zone,
    DROP COLUMN IF EXISTS secondary_player_id,
    DROP COLUMN IF EXISTS team_id,
    DROP COLUMN IF EXISTS event_type_id;

DROP TABLE IF EXISTS public.event_types;
