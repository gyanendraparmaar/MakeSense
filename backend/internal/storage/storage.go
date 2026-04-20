package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store wraps a *sql.DB and offers simple repository methods.
// v0 keeps it single-user; userID stays empty until auth lands.
type Store struct {
	db *sql.DB
}

// Open initializes a SQLite database and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS analyses (
			id TEXT PRIMARY KEY,
			note_id TEXT,
			block_hash TEXT NOT NULL,
			block_text TEXT NOT NULL,
			block_type TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 0,
			structured TEXT NOT NULL,
			model TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_analyses_hash ON analyses(block_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_analyses_note ON analyses(note_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate %q: %w", q, err)
		}
	}
	return nil
}

// ----------------------- Notes -----------------------

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) ListNotes() ([]Note, error) {
	rows, err := s.db.Query(`SELECT id, title, content, created_at, updated_at FROM notes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNote(id string) (*Note, error) {
	var n Note
	err := s.db.QueryRow(`SELECT id, title, content, created_at, updated_at FROM notes WHERE id = ?`, id).
		Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Store) CreateNote(title, content string) (*Note, error) {
	now := time.Now().UTC()
	n := &Note{
		ID:        uuid.NewString(),
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO notes (id, title, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		n.ID, n.Title, n.Content, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (s *Store) UpdateNote(id, title, content string) (*Note, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`UPDATE notes SET title = ?, content = ?, updated_at = ? WHERE id = ?`,
		title, content, now, id,
	)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, nil
	}
	return s.GetNote(id)
}

func (s *Store) DeleteNote(id string) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	return err
}

// ----------------------- Analyses (cache) -----------------------

type CachedAnalysis struct {
	BlockType  string          `json:"type"`
	Confidence float64         `json:"confidence"`
	Structured json.RawMessage `json:"structured"`
	Model      string          `json:"model"`
}

// HashBlock returns the sha256 of a text block — used as cache key.
func HashBlock(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// GetCachedAnalysis returns the most recent analysis for a given block hash, if any.
func (s *Store) GetCachedAnalysis(hash string) (*CachedAnalysis, error) {
	var a CachedAnalysis
	var structured string
	err := s.db.QueryRow(
		`SELECT block_type, confidence, structured, model FROM analyses WHERE block_hash = ? ORDER BY created_at DESC LIMIT 1`,
		hash,
	).Scan(&a.BlockType, &a.Confidence, &structured, &a.Model)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Structured = json.RawMessage(structured)
	return &a, nil
}

// SaveAnalysis persists an analyzer result for caching & audit.
func (s *Store) SaveAnalysis(noteID, blockText, blockType, model string, confidence float64, structured json.RawMessage) error {
	hash := HashBlock(blockText)
	var noteIDArg any = noteID
	if noteID == "" {
		noteIDArg = nil
	}
	_, err := s.db.Exec(
		`INSERT INTO analyses (id, note_id, block_hash, block_text, block_type, confidence, structured, model)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), noteIDArg, hash, blockText, blockType, confidence, string(structured), model,
	)
	return err
}
