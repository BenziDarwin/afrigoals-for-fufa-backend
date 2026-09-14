CREATE TABLE IF NOT EXISTS public.video_analysis_jobs (
    id bigserial PRIMARY KEY,
    match_id bigint NOT NULL,
    requested_by_user_id bigint,
    job_id character varying(128) NOT NULL,
    status character varying(32) DEFAULT 'queued'::character varying NOT NULL,
    original_filename character varying(255),
    model_message text,
    stage character varying(128),
    progress numeric,
    result_json text,
    annotated_video_path text,
    siglip_html_path text,
    video_id bigint,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_video_analysis_jobs_job_id
    ON public.video_analysis_jobs USING btree (job_id);

CREATE INDEX IF NOT EXISTS idx_video_analysis_jobs_match_id
    ON public.video_analysis_jobs USING btree (match_id);

CREATE INDEX IF NOT EXISTS idx_video_analysis_jobs_status
    ON public.video_analysis_jobs USING btree (status);

CREATE INDEX IF NOT EXISTS idx_video_analysis_jobs_video_id
    ON public.video_analysis_jobs USING btree (video_id);
