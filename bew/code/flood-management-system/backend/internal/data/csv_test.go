package data

import (
	"strings"
	"testing"
)

func TestDecodeHistoricalCSVRejectsMissingAndBadTimes(t *testing.T) {
	csv := "station_code,measure_datetime,water_level,quality_flag\nTHA006,2026-09-01 00:10:00,3.23,N\nTHA006,2026-09-01 00:20:00,-999,N\nTHA006,invalid,3.25,N\nTHA006,2026-09-01 00:30:00,null,null\nTHA006,2026-09-01 00:40:00,3.26,N\n"
	rows, err := DecodeHistoricalCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].StationCode != "THA006" || rows[0].LevelMSL != 3.23 || rows[1].LevelMSL != 3.26 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
