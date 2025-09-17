package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const timeFormat = "02.01.2006"
const resultFile = "bsa-schedule.html"

const htmlResultTmpl = `<!DOCTYPE html>
<html><head><link rel="stylesheet" href="bsa-style.css"></head>
<body>
<table class="bsa-table">
<thead>
	<tr>
		<th>Wochentag</th>
		<th>Datum</th>
		<th>Status</th>
	</tr>
</thead>
<tbody>
%v
</tbody>
</table>
</body>
</html>
`
const rowTmpl = `<tr class="%v">
	<td class="td-weekday">%v</td>
	<td class="td-date">%v</td>
	<td class="td-status">%v</td>
</tr>`

type BsaScheduleUntyped struct {
	Weekday interface{} `json:"wochentag"`
	Date    interface{} `json:"datum"`
	State   interface{} `json:"status"`
}

type BsaSchedule struct {
	Weekday string
	Date    time.Time
	State   string
}

func main() {
	googleSpreadsheetApi, ok := os.LookupEnv("GOOGLE_SPREADSHEET_API")
	if !ok {
		log.Fatalf("GOOGLE_SPREADSHEET_API unset")
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v\n", err)
	}

	resp, err := http.DefaultClient.Get(googleSpreadsheetApi)
	if err != nil {
		log.Fatalf("Failed to fetch from %v: %v\n", googleSpreadsheetApi, err)
	}
	defer resp.Body.Close()

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v\n", err)
	}

	var untypedBsaSchedule []BsaScheduleUntyped
	err = json.Unmarshal(read, &untypedBsaSchedule)
	if err != nil {
		log.Fatalf("Failed to unmarshall %v: %v\n", string(read), err)
	}

	var bsaSchedule []BsaSchedule
	for _, scheduleEntry := range untypedBsaSchedule {
		var weekday, rawDate, state string
		var ok bool
		if weekday, ok = scheduleEntry.Weekday.(string); !ok || len(strings.TrimSpace(weekday)) == 0 {
			continue
		}
		if rawDate, ok = scheduleEntry.Date.(string); !ok || len(strings.TrimSpace(rawDate)) == 0 {
			continue
		}
		if state, ok = scheduleEntry.State.(string); !ok || len(strings.TrimSpace(state)) == 0 {
			continue
		}

		date, err := time.Parse(time.RFC3339, rawDate)
		if err != nil {
			log.Printf("Failed to parse time %v: %v\n", rawDate, err)
			continue
		}
		localDate := date.In(berlin)
		if localDate.Hour() != 0 {
			log.Fatalf("Retrieved date is not truncated to day, got %v, local: %v\n", rawDate, localDate)
		}

		bsaSchedule = append(bsaSchedule, BsaSchedule{
			Weekday: weekday,
			Date:    localDate,
			State:   state,
		})
	}

	tableBody := ""
	for _, scheduleEntry := range bsaSchedule {
		log.Printf("Weekday: %v, Date: %v, State: %v\n", scheduleEntry.Weekday, scheduleEntry.Date, scheduleEntry.State)

		tableBody += fmt.Sprintf(rowTmpl, scheduleEntry.State, scheduleEntry.Weekday, scheduleEntry.Date.Format(timeFormat), scheduleEntry.State)
	}

	err = os.WriteFile(resultFile, []byte(fmt.Sprintf(htmlResultTmpl, tableBody)), 0644)
	if err != nil {
		log.Fatalf("Failed to write %v: %v\n", resultFile, err)
	}
}
