package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migrator struct {
	client       *pgxpool.Pool
	migrationDir string
	ctx          context.Context
}

func NewMigrator(client *pgxpool.Pool, migrationDir string, ctx context.Context) *Migrator {
	return &Migrator{client: client, migrationDir: migrationDir, ctx: ctx}
}

func (m *Migrator) Close() {
	m.client.Close()
}

func (m *Migrator) RunMigration() {
	fileNames, err := loadMigrationFileName(m.migrationDir)
	if err != nil {
		return
	}

	migrationTableFile := fileNames[0]
	err = m.checkAndCreateMigrationTable(migrationTableFile)
	if err != nil {
		fmt.Printf("failed to create migration table: %v", err)
		return
	}

	applied, err := m.versionMigrated()
	if err != nil {
		fmt.Printf("failed to read migrated version: %v", err)
		return
	}

	for _, fileName := range fileNames[1:] {
		ver, err := migrationVersionFromFileName(fileName)
		if err != nil {
			fmt.Printf("Failed to read migration version: %v", err)
			return
		}

		if applied[ver] {
			fmt.Printf("migration already present: %v, Skipping...\n", fileName)
			continue
		}

		err = m.applyMigration(fileName, ver)
		if err != nil {
			fmt.Printf("failed to apply migration: %v\n", err)
			return
		}
	}

	fmt.Println("Migration done!")
}

func loadMigrationFileName(migrationDir string) ([]string, error) {
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		fmt.Printf("failed to read migrationDir: %v", err)
		return nil, fmt.Errorf("failed to read migrationDir: %v", err)
	}

	var fileNames []string

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()

		if !strings.HasSuffix(fileName, ".sql") {
			continue
		}

		fileNames = append(fileNames, file.Name())
	}

	slices.Sort(fileNames)

	return fileNames, nil
}

func migrationVersionFromFileName(fileName string) (string, error) {
	before, _, appeared := strings.Cut(fileName, "_")
	if !appeared {
		return "", fmt.Errorf("naming convention does not match: %v", fileName)
	}
	return before, nil
}

func (m *Migrator) checkAndCreateMigrationTable(fileName string) error {
	ver, err := migrationVersionFromFileName(fileName)
	if err != nil {
		return err
	}
	if ver != "000" {
		return fmt.Errorf("invalid migration script file: %v", fileName)
	}
	content, err := os.ReadFile(m.migrationDir + fileName)
	if err != nil {
		fmt.Printf("failed to read file: %v", err)
	}

	_, err = m.client.Exec(m.ctx, string(content))
	if err != nil {
		fmt.Printf("failed to apply migration: %v", fileName)
		return fmt.Errorf("failed to apply migration: %v", fileName)
	}

	return nil
}

func (m *Migrator) versionMigrated() (map[string]bool, error) {
	rows, err := m.client.Query(m.ctx, "select version from schema_migrations;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	verMap := map[string]bool{}

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		verMap[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return verMap, nil
}

func (m *Migrator) applyMigration(fileName string, ver string) error {
	content, err := os.ReadFile(filepath.Join(m.migrationDir, fileName))
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	db, err := m.client.Begin(m.ctx)
	defer db.Rollback(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to initiate transaction: %v", err)
	}
	_, err = db.Exec(m.ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to apply migration: %v", fileName)
	}

	_, err = db.Exec(m.ctx, "insert into schema_migrations (version) values ($1);", ver)
	if err != nil {
		return fmt.Errorf("failed to commit migration version: %v", err)
	}

	err = db.Commit(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	fmt.Printf("Applied: %v\n", fileName)

	return nil
}
