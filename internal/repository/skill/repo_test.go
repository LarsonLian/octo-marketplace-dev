package skill

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Mininglamp-OSS/octo-marketplace/internal/model"
	"github.com/go-sql-driver/mysql"
)

func TestMapDuplicateName(t *testing.T) {
	duplicateName := &mysql.MySQLError{
		Number:  mysqlErrDupEntry,
		Message: "Duplicate entry 'owner-space-name' for key 'skills.uq_skill_owner_space_name_live'",
	}
	if !errors.Is(mapDuplicateName(duplicateName), ErrNameTaken) {
		t.Fatal("skill name constraint violation must map to ErrNameTaken")
	}

	duplicateID := &mysql.MySQLError{
		Number:  mysqlErrDupEntry,
		Message: "Duplicate entry 'id' for key 'skills.PRIMARY'",
	}
	if errors.Is(mapDuplicateName(duplicateID), ErrNameTaken) {
		t.Fatal("unrelated unique constraint must not map to ErrNameTaken")
	}
}

func TestUpdateWithTagsRechecksOwnerAndSpace(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	newName := "new-name"
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE skills SET name = \\? WHERE id = \\? AND owner_id = \\? AND space_id = \\? AND is_deleted = 0").
		WithArgs(newName, "skill-1", "user-1", "space-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = New(db).UpdateWithTags(context.Background(), "skill-1", "space-1", "user-1", UpdateParams{Name: &newName})
	if !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("UpdateWithTags error = %v, want ErrSkillNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSkillAndConsumeTaskRechecksOwnerAndSpace(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	newName := "new-name"
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE parse_tasks SET status = 'consumed'").
		WithArgs("task-1", "user-1", "space-1", "skill-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE skills SET name = \\? WHERE id = \\? AND owner_id = \\? AND space_id = \\? AND is_deleted = 0").
		WithArgs(newName, "skill-1", "user-1", "space-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = New(db).UpdateSkillAndConsumeTask(
		context.Background(),
		"skill-1",
		UpdateParams{Name: &newName},
		"task-1",
		"user-1",
		"space-1",
		"skill-1",
		&model.SkillVersion{ID: "version-1", SkillID: "skill-1", Version: "1.0.1", Storage: string(json.RawMessage(`{}`))},
	)
	if !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("UpdateSkillAndConsumeTask error = %v, want ErrSkillNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
