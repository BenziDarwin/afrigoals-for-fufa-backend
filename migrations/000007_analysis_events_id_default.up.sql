CREATE SEQUENCE IF NOT EXISTS public.analysis_events_id_seq
    AS bigint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.analysis_events_id_seq
    OWNED BY public.analysis_events.id;

SELECT setval(
    'public.analysis_events_id_seq',
    GREATEST(
        COALESCE((SELECT MAX(id) FROM public.analysis_events), 0),
        1
    ),
    true
);

ALTER TABLE public.analysis_events
    ALTER COLUMN id SET DEFAULT nextval('public.analysis_events_id_seq'::regclass);
