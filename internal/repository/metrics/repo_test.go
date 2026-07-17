package metrics

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpsertCounts_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := New(db)

	mock.ExpectExec("INSERT INTO resource_metrics").
		WithArgs("skill", "sk-1", int64(5), int64(2), int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpsertCounts(context.Background(), "skill", "sk-1", 5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestUpsertCounts_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := New(db)

	mock.ExpectExec("INSERT INTO resource_metrics").
		WithArgs("skill", "sk-1", int64(1), int64(0), int64(0)).
		WillReturnError(context.DeadlineExceeded)

	err = repo.UpsertCounts(context.Background(), "skill", "sk-1", 1, 0, 0)
	if err == nil {
		t.Fatal("expected error on DB failure")
	}
}
