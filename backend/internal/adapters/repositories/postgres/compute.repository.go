package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"backend/internal/core/domain"
	"backend/internal/core/ports"
)

type computeRepository struct {
	db *sql.DB
}

// NewComputeRepository returns a Postgres-backed ports.ComputeRepository.
func NewComputeRepository(db *sql.DB) ports.ComputeRepository {
	return &computeRepository{db: db}
}

func (r *computeRepository) Create(ctx context.Context, c *domain.Compute) error {
	const q = `
		INSERT INTO compute (id, user_id, compute_service_id, name, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, q, c.ID, c.UserID, c.ComputeServiceID, c.Name, c.Status, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("create compute: %w", err)
	}
	return nil
}

func (r *computeRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error) {
	const q = `
		SELECT id, user_id, compute_service_id, name, status, created_at
		FROM compute WHERE user_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list compute: %w", err)
	}
	defer rows.Close()

	var out []domain.Compute
	for rows.Next() {
		var c domain.Compute
		if err := rows.Scan(&c.ID, &c.UserID, &c.ComputeServiceID, &c.Name, &c.Status, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan compute: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list compute: %w", err)
	}
	return out, nil
}

func (r *computeRepository) GetByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) (*domain.Compute, error) {
	const q = `
		SELECT id, user_id, compute_service_id, name, status, created_at
		FROM compute WHERE compute_service_id = $1 AND user_id = $2
	`
	var c domain.Compute
	err := r.db.QueryRowContext(ctx, q, serviceID, userID).Scan(
		&c.ID, &c.UserID, &c.ComputeServiceID, &c.Name, &c.Status, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrComputeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get compute: %w", err)
	}
	return &c, nil
}

func (r *computeRepository) DeleteByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) error {
	const q = `DELETE FROM compute WHERE compute_service_id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, q, serviceID, userID)
	if err != nil {
		return fmt.Errorf("delete compute: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete compute: %w", err)
	}
	if n == 0 {
		return domain.ErrComputeNotFound
	}
	return nil
}
