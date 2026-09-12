package archive

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/storage"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

const defaultMaxBufferedRows = 10000

// Writer buffers validated records and writes immutable CSV objects.
// One MQTT subscription fans into this sink; it must never be a second subscriber.
type Writer struct {
	objects       storage.Store
	log           *slog.Logger
	flushRows     int
	flushInterval time.Duration
	maxRows       int
	newID         func() string

	mu    sync.Mutex
	parts map[string]*partBuf
}

type partBuf struct {
	rows [][]string
}

type WriterOptions struct {
	FlushRows     int
	FlushInterval time.Duration
	MaxRows       int
	NewID         func() string
}

func NewWriter(objects storage.Store, log *slog.Logger, opts WriterOptions) *Writer {
	if opts.FlushRows <= 0 {
		opts.FlushRows = 100
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 15 * time.Second
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = defaultMaxBufferedRows
	}
	if opts.NewID == nil {
		opts.NewID = func() string { return uuid.NewString() }
	}
	if log == nil {
		log = slog.Default()
	}
	return &Writer{
		objects:       objects,
		log:           log,
		flushRows:     opts.FlushRows,
		flushInterval: opts.FlushInterval,
		maxRows:       opts.MaxRows,
		newID:         opts.NewID,
		parts:         map[string]*partBuf{},
	}
}

func (w *Writer) Name() string { return "archive" }

func (w *Writer) HandleRecord(ctx context.Context, rec telemetry.Record) error {
	if w == nil || w.objects == nil {
		return nil
	}
	part := PartitionKey(rec)
	w.mu.Lock()
	buf := w.parts[part]
	if buf == nil {
		buf = &partBuf{}
		w.parts[part] = buf
	}
	buf.rows = append(buf.rows, EncodeRow(rec))
	if len(buf.rows) > w.maxRows {
		dropped := len(buf.rows) - w.maxRows
		buf.rows = buf.rows[dropped:]
		w.log.Error("archive buffer overflow; dropping oldest rows", "partition", part, "dropped", dropped)
	}
	shouldFlush := len(buf.rows) >= w.flushRows
	w.mu.Unlock()
	if shouldFlush {
		return w.flushPartition(ctx, part)
	}
	return nil
}

// Run flushes leftover rows on an interval and on shutdown.
func (w *Writer) Run(ctx context.Context) {
	if w == nil {
		return
	}
	t := time.NewTicker(w.flushInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := w.Flush(flushCtx); err != nil {
				w.log.Error("archive final flush failed", "err", err)
			}
			return
		case <-t.C:
			if err := w.Flush(ctx); err != nil {
				w.log.Error("archive timed flush failed", "err", err)
			}
		}
	}
}

func (w *Writer) Flush(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	keys := make([]string, 0, len(w.parts))
	for k := range w.parts {
		keys = append(keys, k)
	}
	w.mu.Unlock()
	var first error
	for _, k := range keys {
		if err := w.flushPartition(ctx, k); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (w *Writer) flushPartition(ctx context.Context, part string) error {
	w.mu.Lock()
	buf := w.parts[part]
	if buf == nil || len(buf.rows) == 0 {
		w.mu.Unlock()
		return nil
	}
	rows := buf.rows
	delete(w.parts, part)
	w.mu.Unlock()

	var body bytes.Buffer
	if err := WriteCSV(&body, rows); err != nil {
		w.requeue(part, rows)
		return err
	}
	key := ObjectKey(part, w.newID())
	if err := w.objects.Put(ctx, key, bytes.NewReader(body.Bytes()), "text/csv"); err != nil {
		w.requeue(part, rows)
		return err
	}
	return nil
}

func (w *Writer) requeue(part string, rows [][]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	existing := w.parts[part]
	if existing == nil {
		w.parts[part] = &partBuf{rows: rows}
		return
	}
	existing.rows = append(rows, existing.rows...)
}

var _ telemetry.Sink = (*Writer)(nil)
