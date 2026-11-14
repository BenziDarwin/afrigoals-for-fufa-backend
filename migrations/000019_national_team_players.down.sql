DROP INDEX IF EXISTS public.idx_players_national_jersey;
ALTER TABLE public.players DROP COLUMN IF EXISTS national_jersey_number;
ALTER TABLE public.players DROP COLUMN IF EXISTS national_team_id;
ALTER TABLE public.clubs DROP CONSTRAINT IF EXISTS clubs_team_type_check;
ALTER TABLE public.clubs DROP COLUMN IF EXISTS team_type;
ALTER TABLE public.clubs DROP COLUMN IF EXISTS country;
-- Retain the positive-number club index so unknown shirt numbers remain valid.
