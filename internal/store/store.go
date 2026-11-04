// Package store keeps everything ugs serves in Postgres (schema ugs): the
// signing key, players and their sessions, lobbies, matchmaker tickets and
// relay allocations. Nothing lives in memory only; a restart loses nothing.
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("ugs: not found")

type Store struct{ pool *pgxpool.Pool }

const schema = `
CREATE SCHEMA IF NOT EXISTS ugs;
CREATE TABLE IF NOT EXISTS ugs.meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS ugs.players (id TEXT PRIMARY KEY, created BIGINT NOT NULL);
CREATE TABLE IF NOT EXISTS ugs.sessions (token TEXT PRIMARY KEY, player TEXT NOT NULL REFERENCES ugs.players(id), created BIGINT NOT NULL);
CREATE TABLE IF NOT EXISTS ugs.lobbies (
	id TEXT PRIMARY KEY, code TEXT NOT NULL, host TEXT NOT NULL, doc JSONB NOT NULL,
	private BOOLEAN NOT NULL, created BIGINT NOT NULL, updated BIGINT NOT NULL);
CREATE INDEX IF NOT EXISTS lobbies_code ON ugs.lobbies(code);
CREATE TABLE IF NOT EXISTS ugs.tickets (
	id TEXT PRIMARY KEY, player TEXT NOT NULL, queue TEXT NOT NULL, doc JSONB NOT NULL,
	match_id TEXT NOT NULL DEFAULT '', created BIGINT NOT NULL);
CREATE INDEX IF NOT EXISTS tickets_queue ON ugs.tickets(queue, created);
CREATE TABLE IF NOT EXISTS ugs.allocations (
	id TEXT PRIMARY KEY, player TEXT NOT NULL, join_code TEXT NOT NULL UNIQUE,
	max_connections INT NOT NULL, region TEXT NOT NULL, created BIGINT NOT NULL);`

// Open connects (a pool of at most 2 connections) and creates the schema.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Ping is the health check.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ID returns n random bytes in hex.
func ID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// SigningKey is the HMAC key of the idTokens; created once and kept.
func (s *Store) SigningKey(ctx context.Context) ([]byte, error) {
	var v string
	err := s.pool.QueryRow(ctx, `SELECT v FROM ugs.meta WHERE k='jwt'`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err = s.pool.Exec(ctx, `INSERT INTO ugs.meta(k,v) VALUES('jwt',$1) ON CONFLICT DO NOTHING`, ID(32)); err != nil {
			return nil, err
		}
		err = s.pool.QueryRow(ctx, `SELECT v FROM ugs.meta WHERE k='jwt'`).Scan(&v)
	}
	return []byte(v), err
}

func now() int64 { return time.Now().Unix() }

// NewPlayer creates an anonymous player and a session token for it.
func (s *Store) NewPlayer(ctx context.Context) (player, session string, err error) {
	player, session = "p"+ID(11), ID(24)
	if _, err = s.pool.Exec(ctx, `INSERT INTO ugs.players(id,created) VALUES($1,$2)`, player, now()); err != nil {
		return
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO ugs.sessions(token,player,created) VALUES($1,$2,$3)`, session, player, now())
	return
}

// PlayerBySession resolves a session token.
func (s *Store) PlayerBySession(ctx context.Context, token string) (string, error) {
	var p string
	err := s.pool.QueryRow(ctx, `SELECT player FROM ugs.sessions WHERE token=$1`, token).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return p, err
}

func (s *Store) PlayerExists(ctx context.Context, id string) bool {
	var n int
	return s.pool.QueryRow(ctx, `SELECT 1 FROM ugs.players WHERE id=$1`, id).Scan(&n) == nil
}

// Doc is a JSON document row (lobby, ticket).
type Doc struct {
	ID, Code, Owner string
	Private         bool
	Body            []byte
	Created         int64
}

func (s *Store) PutLobby(ctx context.Context, l Doc) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ugs.lobbies(id,code,host,doc,private,created,updated) VALUES($1,$2,$3,$4,$5,$6,$6)
		ON CONFLICT(id) DO UPDATE SET doc=$4, host=$3, private=$5, updated=$6`, l.ID, l.Code, l.Owner, l.Body, l.Private, now())
	return err
}

func (s *Store) GetLobby(ctx context.Context, id string) (Doc, error) {
	return s.lobby(ctx, `WHERE id=$1`, id)
}

func (s *Store) LobbyByCode(ctx context.Context, code string) (Doc, error) {
	return s.lobby(ctx, `WHERE code=$1`, code)
}

func (s *Store) lobby(ctx context.Context, where string, arg string) (Doc, error) {
	var d Doc
	err := s.pool.QueryRow(ctx, `SELECT id,code,host,doc,private,created FROM ugs.lobbies `+where, arg).Scan(&d.ID, &d.Code, &d.Owner, &d.Body, &d.Private, &d.Created)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// PublicLobbies lists lobbies updated since cutoff, newest first (the caller filters the documents).
func (s *Store) PublicLobbies(ctx context.Context, cutoff int64, limit int) ([]Doc, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,code,host,doc,private,created FROM ugs.lobbies WHERE NOT private AND updated>=$1 ORDER BY created DESC LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.Code, &d.Owner, &d.Body, &d.Private, &d.Created); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteLobby(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ugs.lobbies WHERE id=$1`, id)
	return err
}

// Touch refreshes a lobby's heartbeat.
func (s *Store) Touch(ctx context.Context, id string) error {
	t, err := s.pool.Exec(ctx, `UPDATE ugs.lobbies SET updated=$2 WHERE id=$1`, id, now())
	if err == nil && t.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Ticket is a matchmaker ticket.
type Ticket struct {
	ID, Player, Queue, MatchID string
	Body                       []byte
	Created                    int64
}

func (s *Store) PutTicket(ctx context.Context, t Ticket) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ugs.tickets(id,player,queue,doc,created) VALUES($1,$2,$3,$4,$5)`, t.ID, t.Player, t.Queue, t.Body, now())
	return err
}

func (s *Store) GetTicket(ctx context.Context, id string) (Ticket, error) {
	var t Ticket
	err := s.pool.QueryRow(ctx, `SELECT id,player,queue,doc,match_id,created FROM ugs.tickets WHERE id=$1`, id).Scan(&t.ID, &t.Player, &t.Queue, &t.Body, &t.MatchID, &t.Created)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Store) DeleteTicket(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ugs.tickets WHERE id=$1`, id)
	return err
}

// Match groups the waiting tickets of a queue into one match once min of them wait
// (or the oldest has waited at least maxWait seconds and there are at least one), up to max.
// It returns the match id assigned to the given ticket, "" while it still waits.
func (s *Store) Match(ctx context.Context, ticket string, min, max int, maxWait int64) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var queue, mine string
	if err := tx.QueryRow(ctx, `SELECT queue, match_id FROM ugs.tickets WHERE id=$1 FOR UPDATE`, ticket).Scan(&queue, &mine); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if mine != "" {
		return mine, nil
	}
	rows, err := tx.Query(ctx, `SELECT id, created FROM ugs.tickets WHERE queue=$1 AND match_id='' ORDER BY created, id LIMIT $2 FOR UPDATE`, queue, max)
	if err != nil {
		return "", err
	}
	var ids []string
	var oldest int64
	for rows.Next() {
		var id string
		var c int64
		if err := rows.Scan(&id, &c); err != nil {
			return "", err
		}
		if len(ids) == 0 {
			oldest = c
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < min && (now()-oldest < maxWait || len(ids) < 1) {
		return "", nil
	}
	m := "m" + ID(8)
	if _, err := tx.Exec(ctx, `UPDATE ugs.tickets SET match_id=$1 WHERE id = ANY($2)`, m, ids); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	// the caller's ticket may have been beyond max; report only if it is in the match
	t, err := s.GetTicket(ctx, ticket)
	return t.MatchID, err
}

// Allocation is a relay allocation.
type Allocation struct {
	ID, Player, JoinCode, Region string
	MaxConnections               int
}

func (s *Store) PutAllocation(ctx context.Context, a Allocation) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ugs.allocations(id,player,join_code,max_connections,region,created) VALUES($1,$2,$3,$4,$5,$6)`,
		a.ID, a.Player, a.JoinCode, a.MaxConnections, a.Region, now())
	return err
}

func (s *Store) AllocationByCode(ctx context.Context, code string) (Allocation, error) {
	var a Allocation
	err := s.pool.QueryRow(ctx, `SELECT id,player,join_code,max_connections,region FROM ugs.allocations WHERE join_code=$1`, code).Scan(&a.ID, &a.Player, &a.JoinCode, &a.MaxConnections, &a.Region)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) AllocationByID(ctx context.Context, id string) (Allocation, error) {
	var a Allocation
	err := s.pool.QueryRow(ctx, `SELECT id,player,join_code,max_connections,region FROM ugs.allocations WHERE id=$1`, id).Scan(&a.ID, &a.Player, &a.JoinCode, &a.MaxConnections, &a.Region)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// Sweep deletes what nobody refreshed: lobbies idle past lobbyTTL seconds, tickets and allocations older than a day.
func (s *Store) Sweep(ctx context.Context, lobbyTTL int64) error {
	t := now()
	for _, q := range []struct {
		sql string
		arg int64
	}{
		{`DELETE FROM ugs.lobbies WHERE updated<$1`, t - lobbyTTL},
		{`DELETE FROM ugs.tickets WHERE created<$1`, t - 86400},
		{`DELETE FROM ugs.allocations WHERE created<$1`, t - 86400},
	} {
		if _, err := s.pool.Exec(ctx, q.sql, q.arg); err != nil {
			return err
		}
	}
	return nil
}
