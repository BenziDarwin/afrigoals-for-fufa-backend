-- Adds player_analysis_reports, introduced with the player analysis report
-- feature. Generated with pg_dump from the schema AutoMigrate produces.

--
--

--
-- Name: player_analysis_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.player_analysis_reports (
    id bigint NOT NULL,
    uuid uuid,
    match_id bigint NOT NULL,
    player_id bigint NOT NULL,
    player_name character varying(255) NOT NULL,
    event_count bigint DEFAULT 0 NOT NULL,
    event_types jsonb DEFAULT '[]'::jsonb NOT NULL,
    score bigint DEFAULT 0 NOT NULL,
    last_event_time_seconds numeric,
    analyst_comment text,
    report_text text NOT NULL,
    event_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    ai_stats_keys jsonb DEFAULT '[]'::jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

--
-- Name: player_analysis_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.player_analysis_reports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: player_analysis_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.player_analysis_reports_id_seq OWNED BY public.player_analysis_reports.id;

--
-- Name: player_analysis_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_analysis_reports ALTER COLUMN id SET DEFAULT nextval('public.player_analysis_reports_id_seq'::regclass);

--
-- Name: player_analysis_reports player_analysis_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_analysis_reports
    ADD CONSTRAINT player_analysis_reports_pkey PRIMARY KEY (id);

--
-- Name: idx_player_analysis_reports_created_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_player_analysis_reports_created_by ON public.player_analysis_reports USING btree (created_by);

--
-- Name: idx_player_analysis_reports_match_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_player_analysis_reports_match_id ON public.player_analysis_reports USING btree (match_id);

--
-- Name: idx_player_analysis_reports_player_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_player_analysis_reports_player_id ON public.player_analysis_reports USING btree (player_id);

--
-- Name: idx_player_analysis_reports_uuid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_player_analysis_reports_uuid ON public.player_analysis_reports USING btree (uuid);

--
--

--
-- leagues.uuid previously had no database default. models.League now declares
-- gen_random_uuid(), matching every other model, so align the column.
--

ALTER TABLE public.leagues ALTER COLUMN uuid SET DEFAULT gen_random_uuid();

--
-- The analysis_events -> analysis_event_stats association is now declared from
-- the parent side, so GORM derives a different constraint name for the same
-- foreign key. Rename it so a schema generated from the models and one built
-- from these migrations stay byte-identical.
--

ALTER TABLE public.analysis_event_stats
    RENAME CONSTRAINT fk_analysis_event_stats_analysis_event TO fk_analysis_events_stats;