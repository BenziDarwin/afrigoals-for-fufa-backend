CREATE TABLE public.player_career_entries (
    id bigserial PRIMARY KEY,
    player_id bigint NOT NULL REFERENCES public.players(id),
    club_id bigint NOT NULL REFERENCES public.clubs(id),
    season text NOT NULL DEFAULT '',
    competition text NOT NULL DEFAULT '',
    start_date date,
    end_date date,
    appearances integer CHECK (appearances >= 0),
    goals integer CHECK (goals >= 0),
    assists integer CHECK (assists >= 0),
    minutes integer CHECK (minutes >= 0),
    source_url text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    created_by bigint NOT NULL REFERENCES public.users(id),
    updated_by bigint NOT NULL REFERENCES public.users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);
CREATE INDEX idx_player_career_entries ON public.player_career_entries(player_id) WHERE deleted_at IS NULL;

CREATE TABLE public.player_highlights (
    id bigserial PRIMARY KEY,
    player_id bigint NOT NULL REFERENCES public.players(id),
    career_entry_id bigint REFERENCES public.player_career_entries(id),
    club_id bigint REFERENCES public.clubs(id),
    title varchar(200) NOT NULL,
    description text NOT NULL DEFAULT '',
    video_url text NOT NULL DEFAULT '',
    object_key text NOT NULL DEFAULT '',
    content_type text NOT NULL DEFAULT '',
    size_bytes bigint NOT NULL DEFAULT 0,
    kind text NOT NULL CHECK (kind IN ('link','upload')),
    status text NOT NULL CHECK (status IN ('pending','ready')),
    created_by bigint NOT NULL REFERENCES public.users(id),
    updated_by bigint NOT NULL REFERENCES public.users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX idx_player_highlights ON public.player_highlights(player_id,status) WHERE deleted_at IS NULL;
