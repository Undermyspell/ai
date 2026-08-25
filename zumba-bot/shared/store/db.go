// Package store bündelt die service-übergreifenden DB-Zugriffe auf die
// Stammtisch-Domäne (Leaderboard, Strafen). Alle Funktionen nehmen ein
// Queryer/Execer entgegen, damit sie mit *sql.DB, *sql.Tx oder den dünnen
// DB-Wrappern der Services funktionieren.
package store

import (
	"context"
	"database/sql"
)

// Queryer wird von *sql.DB und *sql.Tx erfüllt.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Execer wird von *sql.DB und *sql.Tx erfüllt.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// RowQueryer ergänzt Queryer um QueryRowContext – für Abfragen, die höchstens
// eine Zeile liefern (z. B. das Stammtischjahr zu einem Zeitpunkt).
type RowQueryer interface {
	Queryer
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
