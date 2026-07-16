package parse

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Mininglamp-OSS/octo-marketplace/internal/storage"
)

type blockingStorage struct{}

func (blockingStorage) PresignPut(context.Context, string, string, time.Duration) (string, http.Header, error) {
	return "", http.Header{}, nil
}

func (blockingStorage) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (blockingStorage) PublicURL(context.Context, string) (string, error) {
	return "", nil
}

func (blockingStorage) GetObject(ctx context.Context, _ string) (io.ReadCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingStorage) DeleteObject(context.Context, string) error {
	return nil
}

func (blockingStorage) CopyObject(context.Context, string, string) error {
	return nil
}

var _ storage.Storage = (*blockingStorage)(nil)

func TestWorkerMarksTaskFailedAfterParseTimeout(t *testing.T) {
	oldParseTimeout := parseTimeout
	oldStatusUpdateTimeout := statusUpdateTimeout
	parseTimeout = 10 * time.Millisecond
	statusUpdateTimeout = time.Second
	t.Cleanup(func() {
		parseTimeout = oldParseTimeout
		statusUpdateTimeout = oldStatusUpdateTimeout
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE parse_tasks SET status = 'failed', error_code = \\?, error_message = \\? WHERE id = \\?").
		WithArgs("INTERNAL_ERROR", "download failed: context deadline exceeded", "task-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	worker := NewWorker(blockingStorage{}, NewRepo(db), db)
	worker.process("task-1", "skills/upload-1/skill.zip", 1024)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
