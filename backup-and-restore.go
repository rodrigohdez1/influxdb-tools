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

func main() {
	s := flag.String("influxdb-source", "influxdb-source:8088", "Influxdb source where to query and get original database.")
	d := flag.String("influxdb-destination", "influxdb-destination:8088", "Influxdb destination where to store the database.")
	n := flag.String("database", "stress", "Influxdb database name to backup and restore.")
	i := flag.String("database-directory", "/tmp/stress", "Directory to store database.")
	t := flag.Int("since", -1, "Create incremental backup after specified timestamp. Int values.")
	f := flag.Bool("firstrun", false, "Use this flag to execute a first time DB import.")
	w := flag.Int("timeout", 10, "Wait timeout to allow shards to be ready after temp DB restore.")

	flag.Parse()

	// Get variables
	sourceDb := *s
	destinationDb := *d
	databaseDirectory := *i
	dbName := *n
	timeStamp := *t
	firstRun := *f
	timeout := *w

	influxdCommand := "/usr/bin/influxd"
	rmCmd := "/bin/rm"

	if firstRun != false {
		// First time backup
		influxdbImportArgs := []string{"backup", "-portable", "-database", dbName, "-host", sourceDb, databaseDirectory}
		output, err := executeCommand(influxdCommand, influxdbImportArgs)
		fmt.Println(output)
		checkError(err)

		// First time restore
		dbRestoreArgs := []string{"restore", "-portable", "-host", destinationDb, databaseDirectory}
		output, err = executeCommand(influxdCommand, dbRestoreArgs)
		fmt.Println(output)
		checkError(err)
	} else {
		// Backup from specified timeframe
		sinceTime := time.Now().Local().Add(time.Hour * time.Duration(timeStamp)).Format(time.RFC3339)
		partialImportCommand := []string{"backup", "-portable", "-database", dbName, "-host", sourceDb, "-since", sinceTime, databaseDirectory}
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
		sideloadInsertArgs := []string{"-database", tempDB, "-execute", "SELECT * INTO " + dbName + "..:MEASUREMENT FROM /.*/ GROUP BY *"}
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
