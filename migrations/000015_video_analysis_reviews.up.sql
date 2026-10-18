CREATE TABLE IF NOT EXISTS video_analysis_reviews (
    job_id varchar(128) PRIMARY KEY REFERENCES video_analysis_jobs(job_id),
    match_id bigint NOT NULL REFERENCES matches(id),
    version bigint NOT NULL CHECK (version > 0),
    document jsonb NOT NULL,
    updated_by bigint NOT NULL REFERENCES users(id),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_video_analysis_reviews_match ON video_analysis_reviews(match_id);
CREATE TABLE IF NOT EXISTS video_analysis_review_revisions (
    job_id varchar(128) NOT NULL REFERENCES video_analysis_jobs(job_id),
    version bigint NOT NULL CHECK (version > 0),
    document jsonb NOT NULL,
    updated_by bigint NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, version)
);
