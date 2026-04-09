package pages

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPageNotFound = errors.New("page not found")
	ErrTooManyFiles = errors.New("folder already has 100 files")
	ErrTooManyDirs  = errors.New("folder already has 10 subfolders")
)

// PageStore handles page persistence.
type PageStore struct {
	DB *pgxpool.Pool
}

// Upsert creates or updates a page. Enforces folder limits before insert.
func (s *PageStore) Upsert(ctx context.Context, username, folderPath, fileName, contents string) error {
	sizeBytes := len(contents)

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if this is an update (existing page)
	var existingID int64
	err = tx.QueryRow(ctx,
		"SELECT id FROM pages WHERE username = $1 AND folder_path = $2 AND file_name = $3",
		username, folderPath, fileName).Scan(&existingID)
	isUpdate := err == nil

	if !isUpdate {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("checking existing page: %w", err)
		}

		// New page: enforce file count limit
		var fileCount int
		err = tx.QueryRow(ctx,
			"SELECT COUNT(*) FROM pages WHERE username = $1 AND folder_path = $2",
			username, folderPath).Scan(&fileCount)
		if err != nil {
			return fmt.Errorf("counting files: %w", err)
		}
		if fileCount >= 100 {
			return ErrTooManyFiles
		}

		// New page: enforce subfolder count limit in parent folder
		// Check if this folder creates a new subfolder in its parent
		parentFolder := parentPath(folderPath)
		if folderPath != parentFolder { // Only check if we're actually in a subfolder relative to parent
			var dirCount int
			err = tx.QueryRow(ctx,
				`SELECT COUNT(DISTINCT
					split_part(substr(folder_path, length($2) + 1), '/', 1)
				) FROM pages
				WHERE username = $1 AND folder_path LIKE $3 AND folder_path != $2`,
				username, parentFolder, parentFolder+"%").Scan(&dirCount)
			if err != nil {
				return fmt.Errorf("counting subfolders: %w", err)
			}
			// Check if this folder_path already exists; if not, it's a new subfolder
			var existsInFolder int
			err = tx.QueryRow(ctx,
				"SELECT COUNT(*) FROM pages WHERE username = $1 AND folder_path = $2",
				username, folderPath).Scan(&existsInFolder)
			if err != nil {
				return fmt.Errorf("checking folder existence: %w", err)
			}
			if existsInFolder == 0 && dirCount >= 10 {
				return ErrTooManyDirs
			}
		}
	}

	if isUpdate {
		_, err = tx.Exec(ctx,
			`UPDATE pages SET contents = $1, size_bytes = $2, updated_at = now()
			 WHERE id = $3`,
			contents, sizeBytes, existingID)
	} else {
		_, err = tx.Exec(ctx,
			`INSERT INTO pages (username, folder_path, file_name, contents, size_bytes)
			 VALUES ($1, $2, $3, $4, $5)`,
			username, folderPath, fileName, contents, sizeBytes)
	}
	if err != nil {
		return fmt.Errorf("upserting page: %w", err)
	}

	return tx.Commit(ctx)
}

// Get returns the contents of a page.
func (s *PageStore) Get(ctx context.Context, username, folderPath, fileName string) (string, error) {
	var contents string
	err := s.DB.QueryRow(ctx,
		"SELECT contents FROM pages WHERE username = $1 AND folder_path = $2 AND file_name = $3",
		username, folderPath, fileName).Scan(&contents)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPageNotFound
		}
		return "", fmt.Errorf("getting page: %w", err)
	}
	return contents, nil
}

// Delete removes a page. Returns ErrPageNotFound if it doesn't exist.
func (s *PageStore) Delete(ctx context.Context, username, folderPath, fileName string) error {
	tag, err := s.DB.Exec(ctx,
		"DELETE FROM pages WHERE username = $1 AND folder_path = $2 AND file_name = $3",
		username, folderPath, fileName)
	if err != nil {
		return fmt.Errorf("deleting page: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPageNotFound
	}
	return nil
}

// ListFolder returns entries (files and subfolders) for a given user/folder.
func (s *PageStore) ListFolder(ctx context.Context, username, folderPath string) ([]FolderEntry, error) {
	// Get files in this folder
	rows, err := s.DB.Query(ctx,
		"SELECT file_name FROM pages WHERE username = $1 AND folder_path = $2 ORDER BY file_name",
		username, folderPath)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}
	defer rows.Close()

	var entries []FolderEntry
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		entries = append(entries, FolderEntry{Name: name, IsFolder: false})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating files: %w", err)
	}

	// Get direct child subfolders
	subRows, err := s.DB.Query(ctx,
		`SELECT DISTINCT
			CASE
				WHEN folder_path = $2 THEN NULL
				ELSE split_part(substr(folder_path, length($2) + 1), '/', 1)
			END AS subfolder
		 FROM pages
		 WHERE username = $1 AND folder_path LIKE $3 AND folder_path != $2
		 ORDER BY subfolder`,
		username, folderPath, folderPath+"%")
	if err != nil {
		return nil, fmt.Errorf("listing subfolders: %w", err)
	}
	defer subRows.Close()

	// Collect folders first, then prepend them before files
	var folders []FolderEntry
	for subRows.Next() {
		var name *string
		if err := subRows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning subfolder: %w", err)
		}
		if name != nil && *name != "" {
			folders = append(folders, FolderEntry{Name: *name, IsFolder: true})
		}
	}
	if err := subRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subfolders: %w", err)
	}

	// Folders first, then files
	result := make([]FolderEntry, 0, len(folders)+len(entries))
	result = append(result, folders...)
	result = append(result, entries...)
	return result, nil
}

// parentPath returns the parent folder path.
// e.g. "/blog/2026/" -> "/blog/"
// e.g. "/blog/" -> "/"
func parentPath(folderPath string) string {
	if folderPath == "/" {
		return "/"
	}
	// Remove trailing slash, find last slash, keep everything up to and including it
	trimmed := folderPath[:len(folderPath)-1]
	lastSlash := 0
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' {
			lastSlash = i
			break
		}
	}
	return trimmed[:lastSlash+1]
}
