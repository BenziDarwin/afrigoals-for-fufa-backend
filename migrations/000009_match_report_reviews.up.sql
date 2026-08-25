-- Adds match_report_reviews: tracks a match's report through the analyst ->
-- league manager workflow (draft -> submitted -> approved -> distributed,
-- with a submitted -> changes_requested -> submitted loop). One row per
-- match, created lazily by GetMatchReportReview on first view (status
-- defaults to 'draft' - the analyst must explicitly submit before it appears
-- in a league manager's queue). No FK constraints, matching the loose
-- integer-reference convention already used by player_analysis_reports/clips
-- against matches/users.

CREATE TABLE public.match_report_reviews (
    id bigint NOT NULL,
    uuid uuid,
    match_id bigint NOT NULL,
    status character varying(32) DEFAULT 'draft'::character varying NOT NULL,
    submitted_by bigint,
    submitted_at timestamp with time zone,
    reviewed_by bigint,
    reviewed_at timestamp with time zone,
    review_notes text,
    distributed_by bigint,
    distributed_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE public.match_report_reviews_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.match_report_reviews_id_seq OWNED BY public.match_report_reviews.id;

ALTER TABLE ONLY public.match_report_reviews ALTER COLUMN id SET DEFAULT nextval('public.match_report_reviews_id_seq'::regclass);

ALTER TABLE ONLY public.match_report_reviews
    ADD CONSTRAINT match_report_reviews_pkey PRIMARY KEY (id);

-- One row per match: also what the lazy-create logic in
-- getOrCreateMatchReportReview relies on to make concurrent first-views
-- race-safe (a unique-violation on insert means another request already
-- created the row; re-select it).
CREATE UNIQUE INDEX idx_match_report_reviews_match_id ON public.match_report_reviews USING btree (match_id);
CREATE UNIQUE INDEX idx_match_report_reviews_uuid ON public.match_report_reviews USING btree (uuid);
CREATE INDEX idx_match_report_reviews_status ON public.match_report_reviews USING btree (status);
