ALTER TABLE public.clubs ADD COLUMN IF NOT EXISTS team_type text NOT NULL DEFAULT 'club';
ALTER TABLE public.clubs ADD COLUMN IF NOT EXISTS country text NOT NULL DEFAULT '';
ALTER TABLE public.clubs ADD CONSTRAINT clubs_team_type_check CHECK (team_type IN ('club', 'national'));
ALTER TABLE public.players ADD COLUMN IF NOT EXISTS national_team_id bigint REFERENCES public.clubs(id);
ALTER TABLE public.players ADD COLUMN IF NOT EXISTS national_jersey_number integer;
CREATE UNIQUE INDEX idx_players_national_jersey ON public.players (national_team_id, national_jersey_number)
    WHERE deleted_at IS NULL AND national_jersey_number > 0;
-- Some existing databases have no club-shirt index and contain duplicates.
-- Preserve those registrations while enabling national teams. New assignments
-- still pass the application's shirt-number checks.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.players
        WHERE deleted_at IS NULL AND club_id > 0 AND jersey_number > 0
        GROUP BY club_id, jersey_number HAVING count(*) > 1
    ) THEN
        RAISE NOTICE 'Existing duplicate club shirt numbers require reconciliation. Club-shirt index left unchanged.';
    ELSE
        DROP INDEX IF EXISTS public.idx_players_club_jersey;
        CREATE UNIQUE INDEX idx_players_club_jersey ON public.players (club_id, jersey_number)
            WHERE deleted_at IS NULL AND club_id > 0 AND jersey_number > 0;
    END IF;
END $$;
