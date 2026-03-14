package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UpSQL       string `json:"-"`
	DownSQL     string `json:"-"`
}

// MigrationRecord represents a migration record in the database
type MigrationRecord struct {
	Version   int       `db:"version"`
	Name      string    `db:"name"`
	AppliedAt time.Time `db:"applied_at"`
}

// Migrator handles database migrations
type Migrator struct {
	db         *sql.DB
	migrations []*Migration
	tableName  string
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB) *Migrator {
	migrator := &Migrator{
		db:         db,
		tableName:  "schema_migrations",
		migrations: GetAllMigrations(),
	}
	return migrator
}

// Up runs all pending migrations
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	appliedMigrations, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	for _, migration := range m.migrations {
		if m.isMigrationApplied(migration.Version, appliedMigrations) {
			continue
		}

		if err := m.runMigration(ctx, migration, true); err != nil {
			return fmt.Errorf("failed to apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Name)
	}

	return nil
}

// Down rolls back the last migration
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	appliedMigrations, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(appliedMigrations) == 0 {
		return fmt.Errorf("no migrations to roll back")
	}

	// Find the last applied migration
	lastApplied := appliedMigrations[len(appliedMigrations)-1]
	migration := m.findMigration(lastApplied.Version)
	if migration == nil {
		return fmt.Errorf("migration %d not found", lastApplied.Version)
	}

	if err := m.runMigration(ctx, migration, false); err != nil {
		return fmt.Errorf("failed to rollback migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	fmt.Printf("Rolled back migration %d: %s\n", migration.Version, migration.Name)
	return nil
}

// Status returns the current migration status
func (m *Migrator) Status(ctx context.Context) ([]*MigrationStatus, error) {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	appliedMigrations, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	var status []*MigrationStatus
	for _, migration := range m.migrations {
		ms := &MigrationStatus{
			Version:     migration.Version,
			Name:        migration.Name,
			Description: migration.Description,
			Applied:     m.isMigrationApplied(migration.Version, appliedMigrations),
		}

		for _, applied := range appliedMigrations {
			if applied.Version == migration.Version {
				ms.AppliedAt = &applied.AppliedAt
				break
			}
		}

		status = append(status, ms)
	}

	// Sort by version
	sort.Slice(status, func(i, j int) bool {
		return status[i].Version < status[j].Version
	})

	return status, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int        `json:"version"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Applied     bool       `json:"applied"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
}

// ensureMigrationsTable creates the migrations table if it doesn't exist
func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`, m.tableName)

	_, err := m.db.ExecContext(ctx, query)
	return err
}

// getAppliedMigrations returns all applied migrations sorted by version
func (m *Migrator) getAppliedMigrations(ctx context.Context) ([]*MigrationRecord, error) {
	query := fmt.Sprintf("SELECT version, name, applied_at FROM %s ORDER BY version", m.tableName)
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []*MigrationRecord
	for rows.Next() {
		var migration MigrationRecord
		if err := rows.Scan(&migration.Version, &migration.Name, &migration.AppliedAt); err != nil {
			return nil, err
		}
		migrations = append(migrations, &migration)
	}

	return migrations, rows.Err()
}

// isMigrationApplied checks if a migration has been applied
func (m *Migrator) isMigrationApplied(version int, applied []*MigrationRecord) bool {
	for _, migration := range applied {
		if migration.Version == version {
			return true
		}
	}
	return false
}

// findMigration finds a migration by version
func (m *Migrator) findMigration(version int) *Migration {
	for _, migration := range m.migrations {
		if migration.Version == version {
			return migration
		}
	}
	return nil
}

// runMigration executes a migration (up or down)
func (m *Migrator) runMigration(ctx context.Context, migration *Migration, up bool) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if up {
		// Execute up migration
		if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
			return fmt.Errorf("failed to execute up migration SQL: %w", err)
		}

		// Record migration
		recordQuery := fmt.Sprintf("INSERT INTO %s (version, name) VALUES ($1, $2)", m.tableName)
		if _, err := tx.ExecContext(ctx, recordQuery, migration.Version, migration.Name); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	} else {
		// Execute down migration
		if migration.DownSQL != "" {
			if _, err := tx.ExecContext(ctx, migration.DownSQL); err != nil {
				return fmt.Errorf("failed to execute down migration SQL: %w", err)
			}
		}

		// Remove migration record
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE version = $1", m.tableName)
		if _, err := tx.ExecContext(ctx, deleteQuery, migration.Version); err != nil {
			return fmt.Errorf("failed to remove migration record: %w", err)
		}
	}

	return tx.Commit()
}