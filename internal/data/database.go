package facedb

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"lipcoder/face/internal/config"
)

var (
	ErrNotFound = errors.New("face not found")

	dbMu sync.RWMutex
	db   *sql.DB
)

// InitDB 程序启动时调用一次。
// 注意：*sql.DB 本身就是连接池，不是单个连接。
func InitDB(ctx context.Context) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		return nil
	}

	con := config.Load()
	if con.DatabaseURL == "" {
		return errors.New("database url cannot be empty")
	}

	newDB, err := sql.Open("postgres", con.DatabaseURL)
	if err != nil {
		return err
	}

	// 控制数据库并发压力。
	// 不是越大越好，先保守一点。
	newDB.SetMaxOpenConns(10)
	newDB.SetMaxIdleConns(5)
	newDB.SetConnMaxLifetime(30 * time.Minute)
	newDB.SetConnMaxIdleTime(5 * time.Minute)

	if err := newDB.PingContext(ctx); err != nil {
		newDB.Close()
		return err
	}

	db = newDB
	return nil
}

// GetDB 获取长期持有的连接池。
// 如果外面忘了 InitDB，这里会兜底初始化一次。
func GetDB(ctx context.Context) (*sql.DB, error) {
	dbMu.RLock()
	currentDB := db
	dbMu.RUnlock()

	if currentDB != nil {
		return currentDB, nil
	}

	if err := InitDB(ctx); err != nil {
		return nil, err
	}

	dbMu.RLock()
	defer dbMu.RUnlock()

	return db, nil
}

// CloseDB 程序退出时调用一次。
func CloseDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db == nil {
		return nil
	}

	err := db.Close()
	db = nil

	return err
}

// AddFace 新增一张人脸，id 不需要手动传，数据库会自动生成。
func AddFace(ctx context.Context, name string, embedding string) (int64, error) {
	if name == "" {
		return 0, errors.New("name cannot be empty")
	}

	if embedding == "" {
		return 0, errors.New("embedding cannot be empty")
	}

	db, err := GetDB(ctx)
	if err != nil {
		return 0, err
	}

	var id int64

	err = db.QueryRowContext(ctx, `
		INSERT INTO faces (name, embedding)
		VALUES ($1, $2::vector)
		RETURNING id
	`, name, embedding).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

// DeleteFaceByName 按 name 删除人脸。
// 当前 InitFacesTable 里已经给 name 加了 UNIQUE，否则同名会被全部删除。
func DeleteFaceByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	db, err := GetDB(ctx)
	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `
		DELETE FROM faces
		WHERE name = $1
	`, name)

	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// FaceExistsByName 按 name 检查数据库里有没有这个人。
func FaceExistsByName(ctx context.Context, name string) (bool, error) {
	if name == "" {
		return false, errors.New("name cannot be empty")
	}

	db, err := GetDB(ctx)
	if err != nil {
		return false, err
	}

	var exists bool

	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM faces
			WHERE name = $1
		)
	`, name).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}

// SearchFaceByEmbedding 按 embedding 查询最相似的人脸。
// pgvector 的 <=> 是 cosine distance。
// 1 - cosine distance 是 cosine similarity。
// cosine_similarity >= threshold 才认为匹配成功。
func SearchFaceByEmbedding(
	ctx context.Context,
	embedding string,
	threshold float64,
) (string, float64, error) {
	if embedding == "" {
		return "", 0, errors.New("embedding cannot be empty")
	}

	db, err := GetDB(ctx)
	if err != nil {
		return "", 0, err
	}

	var name string
	var similarity float64

	err = db.QueryRowContext(ctx, `
		SELECT
			name,
			1 - distance AS cosine_similarity
		FROM (
			SELECT
				name,
				embedding <=> $1::vector AS distance
			FROM faces
			ORDER BY embedding <=> $1::vector
			LIMIT 1
		) nearest
		WHERE 1 - distance >= $2
	`, embedding, threshold).Scan(
		&name,
		&similarity,
	)

	if err == sql.ErrNoRows {
		return "", 0, ErrNotFound
	}

	if err != nil {
		return "", 0, err
	}

	return name, similarity, nil
}