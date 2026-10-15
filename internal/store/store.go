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

// ErrInvalidSchema reports that a test's score schema breaks its contract, such
// as two keys sharing a stub.
var ErrInvalidSchema = errors.New("invalid schema")

// ErrInvalidResult reports that a result carries a negative run metric.
var ErrInvalidResult = errors.New("invalid result")

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
	return s.db.AutoMigrate(&Repo{}, &Task{}, &Test{}, &Datum{}, &Result{})
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
// stub is empty or not URL-safe, ErrInvalidTest when the test misses a required
// field, and ErrInvalidSchema when the score schema breaks its contract.
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

// SaveDatum stores the JSON a test passes to an agent and returns the datum's
// ref. The ref is derived from the JSON, so saving the same JSON in the same
// test again returns the same ref and leaves one datum. It stores the JSON
// unchanged.
func (s *Store) SaveDatum(org, repo, task, test string, body []byte) (string, error) {
	d := Datum{
		Org: org, Repo: repo, Task: task, Test: test,
		Ref:  DatumRef(body),
		JSON: string(body),
	}
	if err := s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&d).Error; err != nil {
		return "", err
	}
	return d.Ref, nil
}

// GetDatum returns the datum with the given ref in a test, or ErrNotFound.
func (s *Store) GetDatum(org, repo, task, test, ref string) (Datum, error) {
	var d Datum
	if err := s.db.First(&d, "org = ? AND repo = ? AND task = ? AND test = ? AND ref = ?", org, repo, task, test, ref).Error; err != nil {
		return Datum{}, notFound(err)
	}
	return d, nil
}

// ListData returns a test's data, ordered by ref.
func (s *Store) ListData(org, repo, task, test string) ([]Datum, error) {
	var data []Datum
	if err := s.db.Where("org = ? AND repo = ? AND task = ? AND test = ?", org, repo, task, test).Order("ref").Find(&data).Error; err != nil {
		return nil, err
	}
	return data, nil
}

// SaveResult stores the JSON an agent produced for a datum and returns the
// result's ref. The ref is derived from the JSON within its datum, so saving
// the same JSON for the same datum again returns the same ref and leaves one
// result. It returns ErrInvalidResult when a run metric is negative.
func (s *Store) SaveResult(r Result) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	r.Ref = ResultRef(r)
	if err := s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error; err != nil {
		return "", err
	}
	return r.Ref, nil
}

// GetResult returns the result with the given ref in a repo, or ErrNotFound.
func (s *Store) GetResult(org, repo, ref string) (Result, error) {
	var r Result
	if err := s.db.First(&r, "org = ? AND repo = ? AND ref = ?", org, repo, ref).Error; err != nil {
		return Result{}, notFound(err)
	}
	return r, nil
}

// ListResults returns a repo's results, ordered by ref.
func (s *Store) ListResults(org, repo string) ([]Result, error) {
	var results []Result
	if err := s.db.Where("org = ? AND repo = ?", org, repo).Order("ref").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
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

// MoveTest renames the test with the given stubs to newStub and returns the
// moved test. It returns ErrNotFound when the test is missing, ErrConflict when
// a test in the same task already holds newStub, and ErrInvalidStub when newStub
// is empty or not URL-safe.
func (s *Store) MoveTest(org, repo, task, stub, newStub string) (Test, error) {
	t, err := s.GetTest(org, repo, task, stub)
	if err != nil {
		return Test{}, err
	}
	if !ValidStub(newStub) {
		return Test{}, ErrInvalidStub
	}
	if _, err := s.GetTest(org, repo, task, newStub); err == nil {
		return Test{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return Test{}, err
	}

	err = s.db.Model(&Test{}).
		Where("org = ? AND repo = ? AND task = ? AND stub = ?", org, repo, task, stub).
		Update("stub", newStub).Error
	if err != nil {
		return Test{}, err
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
