package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MustPool(ctx context.Context) *pgxpool.Pool {

	conn := "postgres://dbuser:password@localhost:5432/dbname?search_path=public&sslmode=disable"

	if env := os.Getenv("PG_CONN"); env != "" {
		conn = env
	}

	pool, err := pgxpool.New(ctx, conn)
	if err != nil {
		panic(err)
	}
	return pool
}
