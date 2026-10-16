package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultGraphURL = "https://api-v3.thaiwater.net/api/v1/thaiwater30/public/waterlevel_graph"

type graphEnvelope struct {
	Result string `json:"result"`
	Data   struct {
		GraphData []struct {
			DateTime string `json:"datetime"`
			Value    Number `json:"value"`
		} `json:"graph_data"`
	} `json:"data"`
}

func DecodeGraph(reader io.Reader, stationID int64) ([]HistoricalRow, error) {
	var body graphEnvelope
	if err := json.NewDecoder(io.LimitReader(reader, 5<<20)).Decode(&body); err != nil {
		return nil, err
	}
	if body.Result != "OK" || body.Data.GraphData == nil {
		return nil, errors.New("graph source did not return graph_data OK")
	}
	rows := make([]HistoricalRow, 0, len(body.Data.GraphData))
	for _, point := range body.Data.GraphData {
		level, valid := point.Value.Float()
		at, err := time.ParseInLocation("2006-01-02 15:04", point.DateTime, bangkok)
		if !valid || level < -100 || level > 1000 || err != nil || at.After(time.Now().Add(10*time.Minute)) {
			continue
		}
		rows = append(rows, HistoricalRow{StationID: stationID, ObservedAt: at.UTC(), LevelMSL: level})
	}
	return rows, nil
}

func FetchGraph(ctx context.Context, client *http.Client, baseURL string, stationID int64) ([]HistoricalRow, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("station_type", "tele_waterlevel")
	query.Set("station_id", strconv.FormatInt(stationID, 10))
	query.Set("start_date", time.Now().In(bangkok).AddDate(0, 0, -7).Format("2006-01-02"))
	query.Set("end_date", time.Now().In(bangkok).Format("2006-01-02"))
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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
		return nil, fmt.Errorf("graph source returned HTTP %d", resp.StatusCode)
	}
	return DecodeGraph(resp.Body, stationID)
}
