package data

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDecodeGraphRejectsMissingAndBadTimes(t *testing.T) {
	now := time.Now().In(bangkok).Format("2006-01-02 15:04")
	input := fmt.Sprintf(`{"result":"OK","data":{"graph_data":[{"datetime":%q,"value":3.23},{"datetime":%q,"value":null},{"datetime":"not-a-time","value":3.25},{"datetime":%q,"value":-999}]}}`, now, now, now)
	rows, err := DecodeGraph(strings.NewReader(input), 737)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].StationID != 737 || rows[0].LevelMSL != 3.23 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
