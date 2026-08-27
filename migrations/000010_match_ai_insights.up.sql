-- Adds match_ai_insights: persists every question/answer pair the AI Coach
-- Insights assistant answers for a match's performance report, so answers
-- survive page reloads and are visible to anyone who later views that
-- report (a league or club manager, not just the analyst who asked) -
-- clearly attributed to the AI since this table holds nothing else. No FK
-- constraints, matching the existing loose integer-reference convention
-- used by player_analysis_reports/clips/match_report_reviews against
-- matches/users.

CREATE TABLE public.match_ai_insights (
    id bigint NOT NULL,
    uuid uuid DEFAULT gen_random_uuid(),
    match_id bigint NOT NULL,
    question text NOT NULL,
    answer text NOT NULL,
    asked_by bigint,
    created_at timestamp with time zone
);

CREATE SEQUENCE public.match_ai_insights_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.match_ai_insights_id_seq OWNED BY public.match_ai_insights.id;

ALTER TABLE ONLY public.match_ai_insights ALTER COLUMN id SET DEFAULT nextval('public.match_ai_insights_id_seq'::regclass);

ALTER TABLE ONLY public.match_ai_insights
    ADD CONSTRAINT match_ai_insights_pkey PRIMARY KEY (id);

CREATE INDEX idx_match_ai_insights_match_id ON public.match_ai_insights USING btree (match_id);
CREATE UNIQUE INDEX idx_match_ai_insights_uuid ON public.match_ai_insights USING btree (uuid);
