package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Results contains data from InfluxDB query
type Results struct {
	Results []Result `json:"results"`
}

// Result contains data from InfluxDB query
type Result struct {
	Series []Serie `json:"series"`
}

// Serie contains data from InfluxDB query
type Serie struct {
	Name    string     `json:"name"`
	Columns []string   `json:"columns"`
	Values  [][]string `json:"values"`
}

func executeCommand(c string, a []string) (string, error) {
	log.Info(fmt.Sprintf("Executing command: %s %s", c, strings.Join(a, " ")))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(c, a...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String()
	log.Info(stderr.String())
	return output, err
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func executeQuery(url string, db string, query string, method string) []byte {
	r, err := http.NewRequest(method, url, nil)
	checkError(err)

	q := r.URL.Query()
	q.Add("db", db)
	q.Add("q", query)

	r.URL.RawQuery = q.Encode()

	log.Info("Executing ", method, " for URL: ", r.URL.String())

	r.Header.Set("Accept", "application/json")
	c := http.Client{Timeout: time.Second * 20}

	res, err := c.Do(r)
	checkError(err)

	b, err := ioutil.ReadAll(res.Body)
	checkError(err)

	log.Infof(string(b))
	return b
}

func backupAndRestoreCQs(dbSource string, dbDestination string, db string) {

	getCQ := "SHOW CONTINUOUS QUERIES"

	// Get CQs
	r := executeQuery(dbSource, db, getCQ, http.MethodGet)

	var results Results
	json.Unmarshal(r, &results)

	// POST CQs
	for i := range results.Results[0].Series {
		if results.Results[0].Series[i].Name == db {
			if results.Results[0].Series[i].Values != nil {
				for x := range results.Results[0].Series[i].Values {
					cq := results.Results[0].Series[i].Values[x][1]
					executeQuery(dbDestination, db, cq, http.MethodPost)
				}
			}
		}
	}
}

func main() {
	s := flag.String("influxdb-source", "influxdb-source:8088", "Influxdb source where to query and get original database.")
	d := flag.String("influxdb-destination", "influxdb-destination:8088", "Influxdb destination where to store the database.")
	n := flag.String("database", "stress", "Influxdb database name to backup and restore.")
	i := flag.String("database-directory", "/tmp/stress", "Directory to store database.")
	t := flag.Int("since", -1, "Create incremental backup after specified timestamp. Int values.")
	f := flag.Bool("firstrun", false, "Use this flag to execute a first time DB import.")
	w := flag.Int("timeout", 10, "Wait timeout to allow shards to be ready after temp DB restore.")
	c := flag.Bool("continuous-queries", false, "Copy Continuous Queries from source to destination.")
	j := flag.String("influxdb-query-source", "http://influxdb-source:8086", "Influxdb source where to query via HTTP. Used for CQs")
	k := flag.String("influxdb-query-destination", "http://influxdb-destination:8086", "Influxdb destination where to query via HTTP. Used for CQs")
	a := flag.String("start", "", "Start time for backup")
	b := flag.String("end", "", "End time for backup")

	flag.Parse()

	// Get variables
	sourceDb := *s
	destinationDb := *d
	databaseDirectory := *i
	dbName := *n
	timeStamp := *t
	firstRun := *f
	timeout := *w
	cqs := *c
	sourceQueryDB := *j
	destinationQueryDB := *k
	start := *a
	end := *b

	influxdCommand := "/usr/bin/influxd"
	rmCmd := "/bin/rm"

	if cqs != false {
		backupAndRestoreCQs(sourceQueryDB, destinationQueryDB, dbName)
		os.Exit(0)
	}

	intervalBackup := false

	if start != "" && end != "" {
		intervalBackup = true
	}

	if firstRun != false {
		// First time backup
		influxdbImportArgs := []string{"backup", "-portable", "-database", dbName, "-host", sourceDb, databaseDirectory}
		output, err := executeCommand(influxdCommand, influxdbImportArgs)
		fmt.Println(output)
		checkError(err)

		// First time restore
		dbRestoreArgs := []string{"restore", "-portable", "-db", dbName, "-host", destinationDb, databaseDirectory}
		output, err = executeCommand(influxdCommand, dbRestoreArgs)
		fmt.Println(output)
		checkError(err)
	} else {
		// Backup from specified timeframe
		sinceTime := ""
		endTime := ""
		if intervalBackup != false {
			sinceTime = start
			endTime = end
		} else {
			sinceTime = time.Now().Local().Add(time.Hour * time.Duration(timeStamp)).Format(time.RFC3339)
			endTime = time.Now().Local().Format(time.RFC3339)
		}
		partialImportCommand := []string{"backup", "-portable", "-database", dbName, "-host", sourceDb, "-start", sinceTime, "-end", endTime, databaseDirectory}
		output, err := executeCommand(influxdCommand, partialImportCommand)
		checkError(err)
		fmt.Println(output)

		// Restore to tmp DB
		tempDB := dbName + "_tmp"
		restoreToTempDBArgs := []string{"restore", "-portable", "-db", dbName, "-newdb", tempDB, "-host", destinationDb, databaseDirectory}
		output, err = executeCommand(influxdCommand, restoreToTempDBArgs)
		checkError(err)

		// Sleep
		log.Info(fmt.Sprintf("Sleeping for %d seconds to avoid 'shard is disabled' error", timeout))
		time.Sleep(time.Second * time.Duration(timeout))

		// Sideload data into original database
		influxCommand := "influx"
		sideloadInsertArgs := []string{"-database", tempDB, "-execute", "SELECT * INTO " + dbName + "..:MEASUREMENT FROM /.*/ WHERE time > '" + sinceTime + "' and time < '" + endTime + "' GROUP BY *"}

		output, err = executeCommand(influxCommand, sideloadInsertArgs)
		checkError(err)

		// Remove tmp DB
		dropTempDBArgs := []string{"-execute", "DROP DATABASE " + tempDB}
		output, err = executeCommand(influxCommand, dropTempDBArgs)
		checkError(err)
	}

	// Remove tmp directory
	rmArgs := []string{"-rfv", databaseDirectory}
	output, err := executeCommand(rmCmd, rmArgs)
	checkError(err)
	fmt.Println(output)

}
