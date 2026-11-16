CREATE TABLE public.team_match_reports (
 id bigserial PRIMARY KEY,
 match_id bigint NOT NULL REFERENCES public.matches(id),
 club_id bigint NOT NULL REFERENCES public.clubs(id),
 status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','submitted','approved','changes_requested','distributed')),
 summary text NOT NULL DEFAULT '',
 strengths text NOT NULL DEFAULT '',
 weaknesses text NOT NULL DEFAULT '',
 recommendations text NOT NULL DEFAULT '',
 snapshot jsonb NOT NULL DEFAULT '{}',
 authored_by bigint NOT NULL REFERENCES public.users(id),
 submitted_at timestamptz,
 reviewed_by bigint REFERENCES public.users(id),
 reviewed_at timestamptz,
 review_notes text NOT NULL DEFAULT '',
 distributed_by bigint REFERENCES public.users(id),
 distributed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT idx_team_match_report UNIQUE(match_id,club_id)
);
CREATE INDEX idx_team_match_report_status ON public.team_match_reports(status,club_id);
