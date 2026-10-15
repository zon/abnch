package store

// Repo is a repository the service tracks. It is identified by its org stub and
// repo stub, so two repos with the same stubs are the same repo whatever their
// URL.
type Repo struct {
	Org  string `gorm:"primaryKey"`
	Stub string `gorm:"primaryKey"`
	URL  string
}

// Task is a use case within a repo. It is identified by its repo and its stub.
type Task struct {
	Org         string `gorm:"primaryKey"`
	Repo        string `gorm:"primaryKey"`
	Stub        string `gorm:"primaryKey"`
	Name        string
	Description string
}
