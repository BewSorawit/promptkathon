package data

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDecodeLatestDropsInvalidReadings(t *testing.T) {
	now := time.Now().In(bangkok).Format("2006-01-02 15:04")
	text := fmt.Sprintf(`{"waterlevel_data":{"result":"OK","data":[{"waterlevel_datetime":%q,"waterlevel_msl":"3.23","situation_level":5,"station":{"id":737,"tele_station_oldcode":"THA006","tele_station_name":{"th":"วัดท่าเจดีย์"},"tele_station_lat":14.15,"tele_station_long":100.12,"min_bank":2.14},"geocode":{"province_name":{"th":"สุพรรณบุรี"}}},{"waterlevel_datetime":%q,"waterlevel_msl":"-999","station":{"id":738,"tele_station_name":{"th":"bad"},"tele_station_lat":14.15,"tele_station_long":100.12}}]}}`, now, now)
	readings, err := DecodeLatest(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1 || readings[0].StationID != 737 || readings[0].Code != "THA006" {
		t.Fatalf("unexpected readings: %+v", readings)
	}
}
