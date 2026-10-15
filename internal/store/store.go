package store

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound reports that a requested repo or task does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidStub reports that a repo's org stub or repo stub is empty or not
// URL-safe.
var ErrInvalidStub = errors.New("invalid stub")

// ErrConflict reports that a write would collide with an existing record.
var ErrConflict = errors.New("conflict")

// ErrInvalidTest reports that a test is missing a required field.
var ErrInvalidTest = errors.New("invalid test")

// Store reads and writes the service's domain data in Postgres.
type Store struct {
	db *gorm.DB
}

// New returns a store over an open database.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the tables the store needs.
func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&Repo{}, &Task{}, &Test{})
}

// Close closes the store's database connection.
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// SaveRepo writes a repo, creating it or replacing the stored repo with the
// same org stub and repo stub. It returns ErrInvalidStub when the org stub or
// repo stub is empty or not URL-safe.
func (s *Store) SaveRepo(r Repo) error {
	if !ValidStub(r.Org) || !ValidStub(r.Stub) {
		return ErrInvalidStub
	}
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error
}

// GetRepo returns the repo with the given stubs, or ErrNotFound.
func (s *Store) GetRepo(org, stub string) (Repo, error) {
	var r Repo
	if err := s.db.First(&r, "org = ? AND stub = ?", org, stub).Error; err != nil {
		return Repo{}, notFound(err)
	}
	return r, nil
}

// ListRepos returns every repo, ordered by org stub and repo stub.
func (s *Store) ListRepos() ([]Repo, error) {
	var repos []Repo
	if err := s.db.Order("org, stub").Find(&repos).Error; err != nil {
		return nil, err
	}
	return repos, nil
}

// SaveTask writes a task, creating it or replacing the stored task with the
// same repo and stub. It returns ErrInvalidStub when the task stub is empty or
// not URL-safe.
func (s *Store) SaveTask(t Task) error {
	if !ValidStub(t.Stub) {
		return ErrInvalidStub
	}
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&t).Error
}

// GetTask returns the task with the given repo and stub, or ErrNotFound.
func (s *Store) GetTask(org, repo, stub string) (Task, error) {
	var t Task
	if err := s.db.First(&t, "org = ? AND repo = ? AND stub = ?", org, repo, stub).Error; err != nil {
		return Task{}, notFound(err)
	}
	return t, nil
}

// ListTasks returns a repo's tasks, ordered by stub.
func (s *Store) ListTasks(org, repo string) ([]Task, error) {
	var tasks []Task
	if err := s.db.Where("org = ? AND repo = ?", org, repo).Order("stub").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// SaveTest writes a test, creating it or replacing the stored test with the
// same repo, task stub, and test stub. It returns ErrInvalidStub when the test
// stub is empty or not URL-safe, and ErrInvalidTest when the test misses a
// required field.
func (s *Store) SaveTest(t Test) error {
	if !ValidStub(t.Stub) {
		return ErrInvalidStub
	}
	if err := t.Validate(); err != nil {
		return err
	}
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&t).Error
}

// GetTest returns the test with the given repo, task stub, and test stub, or
// ErrNotFound.
func (s *Store) GetTest(org, repo, task, stub string) (Test, error) {
	var t Test
	if err := s.db.First(&t, "org = ? AND repo = ? AND task = ? AND stub = ?", org, repo, task, stub).Error; err != nil {
		return Test{}, notFound(err)
	}
	return t, nil
}

// ListTests returns a task's tests, ordered by stub.
func (s *Store) ListTests(org, repo, task string) ([]Test, error) {
	var tests []Test
	if err := s.db.Where("org = ? AND repo = ? AND task = ?", org, repo, task).Order("stub").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

// MoveTask renames the task with the given stub to newStub and returns the
// moved task. It returns ErrNotFound when the task is missing, ErrConflict when
// a task already holds newStub, and ErrInvalidStub when newStub is empty or not
// URL-safe.
func (s *Store) MoveTask(org, repo, stub, newStub string) (Task, error) {
	t, err := s.GetTask(org, repo, stub)
	if err != nil {
		return Task{}, err
	}
	if !ValidStub(newStub) {
		return Task{}, ErrInvalidStub
	}
	if _, err := s.GetTask(org, repo, newStub); err == nil {
		return Task{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return Task{}, err
	}

	err = s.db.Model(&Task{}).
		Where("org = ? AND repo = ? AND stub = ?", org, repo, stub).
		Update("stub", newStub).Error
	if err != nil {
		return Task{}, err
	}
	t.Stub = newStub
	return t, nil
}

// notFound maps Gorm's record-not-found error to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
