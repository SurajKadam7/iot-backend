package exportjob

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/surajkadam7/iot-backend/internal/archive"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
	"github.com/surajkadam7/iot-backend/internal/storage"
)

// Worker claims queued export_jobs and streams a tenant-scoped CSV into the object store.
type Worker struct {
	repo         repository.Store
	objects      storage.Store
	log          *slog.Logger
	poll         time.Duration
	exportPrefix string
}

func New(repo repository.Store, objects storage.Store, log *slog.Logger, exportPrefix string, poll time.Duration) *Worker {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	if exportPrefix == "" {
		exportPrefix = "exports"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker{repo: repo, objects: objects, log: log, poll: poll, exportPrefix: exportPrefix}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil {
		return
	}
	t := time.NewTicker(w.poll)
	defer t.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	if _, err := w.ProcessQueued(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.log.Error("export worker", "err", err)
	}
}

// ProcessQueued claims and runs every queued job. Tests call this synchronously.
func (w *Worker) ProcessQueued(ctx context.Context) (int, error) {
	if w == nil || w.objects == nil {
		return 0, nil
	}
	n := 0
	for {
		job, err := w.repo.ClaimNextExportJob(ctx)
		if errors.Is(err, repository.ErrNotFound) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if err := w.process(ctx, job); err != nil {
			w.log.Error("export job failed", "id", job.ID, "organization_id", job.OrganizationID, "err", err)
			_ = w.repo.FinishExportJob(ctx, job.ID, models.ExportFailed, "", "failed to assemble export")
		}
		n++
	}
}

func (w *Worker) process(ctx context.Context, job models.ExportJob) error {
	q := archive.Query{
		OrganizationID: job.OrganizationID,
		DeviceIDs:      job.DeviceIDs,
		From:           job.FromTS,
		To:             job.ToTS,
	}
	keys, err := archive.ListMatching(ctx, w.objects, q)
	if err != nil {
		return err
	}
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		err := archive.MergeCSV(ctx, w.objects, keys, q, pw)
		_ = pw.CloseWithError(err)
		errCh <- err
	}()
	objectKey := archive.ExportObjectKey(w.exportPrefix, job.OrganizationID, job.ID)
	putErr := w.objects.Put(ctx, objectKey, pr, "text/csv")
	mergeErr := <-errCh
	if putErr != nil {
		return putErr
	}
	if mergeErr != nil {
		return mergeErr
	}
	return w.repo.FinishExportJob(ctx, job.ID, models.ExportSucceeded, objectKey, "")
}
