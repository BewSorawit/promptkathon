package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"flood-management-system/backend/internal/data"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema embed.FS

type Store struct{ DB *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	sql, err := schema.ReadFile("schema.sql")
	if err != nil {
		pool.Close()
		return nil, err
	}
	if _, err = pool.Exec(ctx, string(sql)); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{DB: pool}, nil
}

func (s *Store) SaveReadings(ctx context.Context, readings []data.Reading) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range readings {
		_, err = tx.Exec(ctx, `INSERT INTO stations (id,code,name,province,district,river,latitude,longitude,bank_msl,situation_level,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,province=EXCLUDED.province,district=EXCLUDED.district,river=EXCLUDED.river,latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude,bank_msl=EXCLUDED.bank_msl,situation_level=EXCLUDED.situation_level,updated_at=EXCLUDED.updated_at
		WHERE stations.updated_at <= EXCLUDED.updated_at`, r.StationID, r.Code, r.Name, r.Province, r.District, r.River, r.Latitude, r.Longitude, r.BankMSL, r.Situation, r.ObservedAt)
		if err != nil {
			return fmt.Errorf("save station %d: %w", r.StationID, err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO observations (station_id,observed_at,level_msl,source) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, r.StationID, r.ObservedAt, r.LevelMSL, data.SourceName)
		if err != nil {
			return fmt.Errorf("save reading %d: %w", r.StationID, err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ImportHistory(ctx context.Context, rows []data.HistoricalRow, source string) (int, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	count := 0
	for _, row := range rows {
		tag, err := tx.Exec(ctx, `INSERT INTO observations (station_id,observed_at,level_msl,source) SELECT id,$3,$4,$5 FROM stations WHERE (code=$2 AND $2 <> '') OR (id=$1 AND $2 = '') ON CONFLICT DO NOTHING`, row.StationID, row.StationCode, row.ObservedAt, row.LevelMSL, source)
		if err != nil {
			return 0, err
		}
		count += int(tag.RowsAffected())
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ClaimHistoryFetch(ctx context.Context, stationID int64) (bool, error) {
	tag, err := s.DB.Exec(ctx, `INSERT INTO history_syncs (station_id,attempted_at) VALUES ($1,now()) ON CONFLICT (station_id) DO UPDATE SET attempted_at=now() WHERE history_syncs.attempted_at < now() - interval '10 minutes'`, stationID)
	return tag.RowsAffected() == 1, err
}

func (s *Store) RecordSync(ctx context.Context, attempted time.Time, count int, syncErr error) error {
	status, detail := "ok", ""
	var success *time.Time
	if syncErr != nil {
		status, detail = "error", syncErr.Error()
	} else {
		success = &attempted
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO source_syncs (source,attempted_at,succeeded_at,status,detail,received_count) VALUES ($1,$2,$3,$4,$5,$6)
	ON CONFLICT (source) DO UPDATE SET attempted_at=EXCLUDED.attempted_at,succeeded_at=COALESCE(EXCLUDED.succeeded_at,source_syncs.succeeded_at),status=EXCLUDED.status,detail=EXCLUDED.detail,received_count=EXCLUDED.received_count`, data.SourceName, attempted, success, status, detail, count)
	return err
}

type Station struct {
	ID             int64      `json:"id"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	Province       string     `json:"province"`
	District       string     `json:"district"`
	River          string     `json:"river"`
	Latitude       float64    `json:"latitude"`
	Longitude      float64    `json:"longitude"`
	BankMSL        *float64   `json:"bank_msl"`
	SituationLevel *int       `json:"situation_level"`
	ObservedAt     *time.Time `json:"observed_at"`
	LevelMSL       *float64   `json:"level_msl"`
	IsStale        bool       `json:"is_stale"`
}

const stationSelect = `SELECT s.id,s.code,s.name,s.province,s.district,s.river,s.latitude,s.longitude,s.bank_msl,s.situation_level,o.observed_at,o.level_msl
FROM stations s LEFT JOIN LATERAL (SELECT observed_at,level_msl FROM observations WHERE station_id=s.id ORDER BY observed_at DESC LIMIT 1) o ON true`

func scanStation(row pgx.Row) (Station, error) {
	var station Station
	err := row.Scan(&station.ID, &station.Code, &station.Name, &station.Province, &station.District, &station.River, &station.Latitude, &station.Longitude, &station.BankMSL, &station.SituationLevel, &station.ObservedAt, &station.LevelMSL)
	station.IsStale = station.ObservedAt == nil || time.Since(*station.ObservedAt) > 30*time.Minute
	return station, err
}

func (s *Store) Stations(ctx context.Context, province string) ([]Station, error) {
	query := stationSelect
	query += ` WHERE s.province <> 'สาธารณรัฐแห่งสหภาพเมียนมา'`
	args := []any{}
	if province != "" {
		query += ` AND s.province=$1`
		args = append(args, province)
	}
	query += ` ORDER BY s.province,s.name`
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stations := []Station{}
	for rows.Next() {
		st, err := scanStation(rows)
		if err != nil {
			return nil, err
		}
		stations = append(stations, st)
	}
	return stations, rows.Err()
}

func (s *Store) Station(ctx context.Context, id int64) (Station, error) {
	return scanStation(s.DB.QueryRow(ctx, stationSelect+` WHERE s.id=$1`, id))
}

type Observation struct {
	ObservedAt time.Time `json:"observed_at"`
	LevelMSL   float64   `json:"level_msl"`
	Source     string    `json:"source"`
}

func (s *Store) Observations(ctx context.Context, id int64, since time.Time) ([]Observation, error) {
	rows, err := s.DB.Query(ctx, `SELECT observed_at,level_msl,source FROM observations WHERE station_id=$1 AND observed_at >= $2 ORDER BY observed_at`, id, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Observation{}
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ObservedAt, &o.LevelMSL, &o.Source); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

type Forecast struct {
	OriginAt     time.Time `json:"origin_at"`
	TargetAt     time.Time `json:"target_at"`
	PredictedMSL float64   `json:"predicted_msl"`
	LowerMSL     float64   `json:"lower_msl"`
	UpperMSL     float64   `json:"upper_msl"`
	ModelRunID   int64     `json:"model_run_id"`
}

func (s *Store) Forecast(ctx context.Context, id int64) (*Forecast, error) {
	var f Forecast
	err := s.DB.QueryRow(ctx, `SELECT origin_at,target_at,predicted_msl,lower_msl,upper_msl,model_run_id FROM forecasts WHERE station_id=$1 ORDER BY origin_at DESC LIMIT 1`, id).Scan(&f.OriginAt, &f.TargetAt, &f.PredictedMSL, &f.LowerMSL, &f.UpperMSL, &f.ModelRunID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

type Sync struct {
	Source        string     `json:"source"`
	AttemptedAt   time.Time  `json:"attempted_at"`
	SucceededAt   *time.Time `json:"succeeded_at"`
	Status        string     `json:"status"`
	Detail        string     `json:"detail"`
	ReceivedCount int        `json:"received_count"`
}

func (s *Store) SourceSync(ctx context.Context) (*Sync, error) {
	var sync Sync
	err := s.DB.QueryRow(ctx, `SELECT source,attempted_at,succeeded_at,status,detail,received_count FROM source_syncs WHERE source=$1`, data.SourceName).Scan(&sync.Source, &sync.AttemptedAt, &sync.SucceededAt, &sync.Status, &sync.Detail, &sync.ReceivedCount)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sync, nil
}
