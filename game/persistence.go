package game

// Persistence saves, loads and lists the saves of the active Stage's ECS and resources.
type Persistence interface {
	// List returns every save found for basePath, "" (quicksave) first.
	List(basePath string) ([]string, error)

	// Save writes resources, every tracked Serializable and the ECS snapshot under basePath/label.
	Save(basePath, label string, resources ...any) error

	// Load restores a snapshot written by Save, leaving anything absent from it unchanged.
	Load(basePath, label string, resources ...any) error
}
