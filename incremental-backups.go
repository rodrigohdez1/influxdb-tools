package main

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
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

func main() {

	s := flag.String("influxdb-source", "influxdb-source:8088", "Influxdb source where to query and get original database.")
	i := flag.String("influxdb-destination", "influxdb-destination:8088", "Influxdb destination where to store the database.")
	n := flag.String("database", "stress", "Influxdb database name to backup and restore.")
	r := flag.String("start", "2006-01-00T00:00:00Z", "Timeframe in RFC3339 format to start backup.")
	l := flag.String("end", "2006-01-00T00:00:00Z", "Timeframe in RFC3339 format to start backup.")
	c := flag.String("command", "/opt/influxdb/backup-and-restore", "Backup and restore command path.")

	flag.Parse()

	sourceDb := *s
	destinationDb := *i
	dbName := *n
	initialDate := *r
	endDate := *l
	command := *c
	backupTimeframe := 1
	initial, err := time.Parse(time.RFC3339, initialDate)
	until, err := time.Parse(time.RFC3339, endDate)
	checkError(err)

	d := until.Sub(initial)
	log.Info(fmt.Sprintf("Initiating backups from %s to %s using %d %d-hour per batch backups", initialDate, endDate, int(d.Hours()), backupTimeframe))
	start, err := time.Parse(time.RFC3339, initialDate)
	checkError(err)

	for i := 0; i < int(d.Hours()); i++ {
		n := start.Add(time.Hour * time.Duration(backupTimeframe))
		backupCommand := []string{"-start", start.Format(time.RFC3339), "-end", n.Format(time.RFC3339), "-influxdb-source", sourceDb, "-influxdb-destination", destinationDb, "-database", dbName}
		output, err := executeCommand(command, backupCommand)
		checkError(err)
		log.Info(output)
		start = n
	}
}
