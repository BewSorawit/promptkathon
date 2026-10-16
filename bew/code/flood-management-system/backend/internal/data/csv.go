package data

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type HistoricalRow struct {
	StationID   int64
	StationCode string
	ObservedAt  time.Time
	LevelMSL    float64
}

func DecodeHistoricalCSV(reader io.Reader) ([]HistoricalRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	indices := map[string]int{}
	for i, name := range header {
		indices[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))] = i
	}
	find := func(options ...string) (int, error) {
		for _, name := range options {
			if i, ok := indices[name]; ok {
				return i, nil
			}
		}
		return 0, fmt.Errorf("missing CSV column: %s", strings.Join(options, "/"))
	}
	idCol, hasID := indices["station_id"]
	if !hasID {
		idCol, hasID = indices["tele_station_id"]
	}
	codeCol, hasCode := indices["station_code"]
	if !hasID && !hasCode {
		return nil, fmt.Errorf("missing CSV column: station_id/station_code")
	}
	timeCol, err := find("observed_at", "waterlevel_datetime", "measure_datetime", "datetime")
	if err != nil {
		return nil, err
	}
	levelCol, err := find("level_msl", "waterlevel_msl", "water_level")
	if err != nil {
		return nil, err
	}
	qualityCol, hasQuality := indices["quality_flag"]
	rows := []HistoricalRow{}
	for line := 2; ; line++ {
		fields, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("CSV line %d: %w", line, readErr)
		}
		if timeCol >= len(fields) || levelCol >= len(fields) || (hasID && idCol >= len(fields)) || (hasCode && codeCol >= len(fields)) || (hasQuality && qualityCol >= len(fields)) {
			continue
		}
		if hasQuality && strings.TrimSpace(fields[qualityCol]) != "N" {
			continue
		}
		var id int64
		var code string
		if hasID {
			id, _ = strconv.ParseInt(strings.TrimSpace(fields[idCol]), 10, 64)
		}
		if hasCode {
			code = strings.TrimSpace(fields[codeCol])
		}
		level, valid := Number(strings.TrimSpace(fields[levelCol])).Float()
		if (id <= 0 && code == "") || !valid || level < -100 || level > 1000 {
			continue
		}
		timeText := strings.TrimSpace(fields[timeCol])
		at, parseErr := time.Parse(time.RFC3339, timeText)
		if parseErr != nil {
			at, parseErr = time.ParseInLocation("2006-01-02 15:04:05", timeText, bangkok)
		}
		if parseErr != nil {
			at, parseErr = time.ParseInLocation("2006-01-02 15:04", timeText, bangkok)
		}
		if parseErr != nil || at.After(time.Now().Add(10*time.Minute)) {
			continue
		}
		rows = append(rows, HistoricalRow{StationID: id, StationCode: code, ObservedAt: at.UTC(), LevelMSL: level})
	}
	return rows, nil
}
