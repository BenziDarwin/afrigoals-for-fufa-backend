package services

import (
	"sync"

	"afrigoals.com/database"
)

var (
	clipStatusSchemaOnce sync.Once
	clipStatusSchemaErr  error
)

func ensureClipStatusColumns() error {
	clipStatusSchemaOnce.Do(func() {
		if err := database.DB.Exec(`
			ALTER TABLE public.clips
				ADD COLUMN IF NOT EXISTS status character varying(20) NOT NULL DEFAULT 'processing'
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
		if err := database.DB.Exec(`
			ALTER TABLE public.clips
				ADD COLUMN IF NOT EXISTS error_message character varying(500)
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
		if err := database.DB.Exec(`
			ALTER TABLE public.clips
				ADD COLUMN IF NOT EXISTS object_key character varying(500)
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
		if err := database.DB.Exec(`
			UPDATE public.clips
			SET status = 'completed'
			WHERE clip_url IS NOT NULL
				AND btrim(clip_url) <> ''
				AND clip_url NOT LIKE '/uploads/clips/%'
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
		if err := database.DB.Exec(`
			UPDATE public.clips
			SET status = 'pending', clip_url = NULL, object_key = NULL
			WHERE clip_url LIKE '/uploads/clips/%'
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
		if err := database.DB.Exec(`
			UPDATE public.clips
			SET status = 'processing'
			WHERE status IS NULL OR status = ''
		`).Error; err != nil {
			clipStatusSchemaErr = err
			return
		}
	})
	return clipStatusSchemaErr
}
