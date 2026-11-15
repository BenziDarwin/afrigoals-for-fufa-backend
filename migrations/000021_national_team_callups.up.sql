CREATE TABLE public.national_callups (
    id bigserial PRIMARY KEY,
    team_id bigint NOT NULL REFERENCES public.clubs(id),
    player_id bigint NOT NULL REFERENCES public.players(id),
    called_by bigint NOT NULL REFERENCES public.users(id),
    called_at timestamptz NOT NULL DEFAULT now(),
    jersey_number integer,
    withdrawn_by bigint REFERENCES public.users(id),
    withdrawn_at timestamptz
);
CREATE UNIQUE INDEX idx_active_national_callup ON public.national_callups(team_id,player_id) WHERE withdrawn_at IS NULL;
CREATE INDEX idx_national_callup_history ON public.national_callups(team_id,called_at DESC);

CREATE FUNCTION public.afrigoals_update_player_callups() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL OR OLD.national_team_id IS DISTINCT FROM NEW.national_team_id THEN
        UPDATE public.national_callups SET withdrawn_at=now()
        WHERE player_id=NEW.id AND withdrawn_at IS NULL
          AND (NEW.deleted_at IS NOT NULL OR team_id IS DISTINCT FROM NEW.national_team_id);
    END IF;
    UPDATE public.national_callups SET jersey_number=NEW.national_jersey_number
    WHERE player_id=NEW.id AND team_id=NEW.national_team_id AND withdrawn_at IS NULL;
    RETURN NEW;
END $$;
CREATE TRIGGER update_player_callups AFTER UPDATE OF nationality,country_code,national_team_id,national_jersey_number,deleted_at ON public.players
    FOR EACH ROW EXECUTE FUNCTION public.afrigoals_update_player_callups();
