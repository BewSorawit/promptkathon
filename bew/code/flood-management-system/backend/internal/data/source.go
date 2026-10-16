package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const SourceName = "ThaiWater"

var bangkok = time.FixedZone("ICT", 7*60*60)

type Number string

func (n *Number) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*n = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*n = Number(s)
		return nil
	}
	*n = Number(raw)
	return nil
}

func (n Number) Float() (float64, bool) {
	v, err := strconv.ParseFloat(string(n), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v == -999 || v == 9999 || v == 999999 {
		return 0, false
	}
	return v, true
}

type rawStation struct {
	ID      int64   `json:"id"`
	Code    string  `json:"tele_station_oldcode"`
	Name    names   `json:"tele_station_name"`
	Lat     float64 `json:"tele_station_lat"`
	Lon     float64 `json:"tele_station_long"`
	MinBank Number  `json:"min_bank"`
}
type names struct {
	Thai string `json:"th"`
}
type rawRecord struct {
	DateTime  string     `json:"waterlevel_datetime"`
	Level     Number     `json:"waterlevel_msl"`
	Situation *int       `json:"situation_level"`
	Station   rawStation `json:"station"`
	Geo       struct {
		Province names `json:"province_name"`
		District names `json:"amphoe_name"`
	} `json:"geocode"`
	River string `json:"river_name"`
}
type envelope struct {
	WaterLevel struct {
		Result string      `json:"result"`
		Data   []rawRecord `json:"data"`
	} `json:"waterlevel_data"`
}

type Reading struct {
	StationID                       int64
	Code                            string
	Name, Province, District, River string
	Latitude, Longitude             float64
	BankMSL                         *float64
	Situation                       *int
	ObservedAt                      time.Time
	LevelMSL                        float64
}

func parseRecord(r rawRecord) (Reading, bool) {
	level, ok := r.Level.Float()
	if !ok || level < -100 || level > 1000 || r.Station.ID <= 0 || strings.TrimSpace(r.Station.Name.Thai) == "" || r.Geo.Province.Thai == "สาธารณรัฐแห่งสหภาพเมียนมา" || r.Station.Lat < 5 || r.Station.Lat > 22 || r.Station.Lon < 97 || r.Station.Lon > 106 {
		return Reading{}, false
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", r.DateTime, bangkok)
	if err != nil || at.After(time.Now().Add(10*time.Minute)) {
		return Reading{}, false
	}
	reading := Reading{StationID: r.Station.ID, Code: r.Station.Code, Name: r.Station.Name.Thai, Province: r.Geo.Province.Thai, District: r.Geo.District.Thai, River: r.River, Latitude: r.Station.Lat, Longitude: r.Station.Lon, Situation: r.Situation, ObservedAt: at.UTC(), LevelMSL: level}
	if bank, valid := r.Station.MinBank.Float(); valid && bank >= -100 && bank <= 1000 {
		reading.BankMSL = &bank
	}
	if reading.Situation != nil && (*reading.Situation < 1 || *reading.Situation > 5) {
		reading.Situation = nil
	}
	return reading, true
}

func DecodeLatest(r io.Reader) ([]Reading, error) {
	var body envelope
	if err := json.NewDecoder(io.LimitReader(r, 10<<20)).Decode(&body); err != nil {
		return nil, err
	}
	if body.WaterLevel.Result != "OK" || body.WaterLevel.Data == nil {
		return nil, errors.New("source did not return waterlevel_data OK")
	}
	readings := make([]Reading, 0, len(body.WaterLevel.Data))
	for _, raw := range body.WaterLevel.Data {
		if reading, ok := parseRecord(raw); ok {
			readings = append(readings, reading)
		}
	}
	if len(readings) == 0 {
		return nil, errors.New("source returned no valid readings")
	}
	return readings, nil
}

func FetchLatest(ctx context.Context, client *http.Client, url string) ([]Reading, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FloodManagementMVP/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	return DecodeLatest(resp.Body)
}
