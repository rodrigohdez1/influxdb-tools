# InfluxDB backup and restore utility
Use this to install the InfluxDB backup and restore script.

# Testing
Examples running in docker-compose.
To get your docker environment running:
1. Create default configuration file. Instructions here: [InfluxDB Docker Hub page](https://hub.docker.com/_/influxdb).
2. Compile script: ```$ go get -v github.com/sirupsen/logrus && go build -v .```
3. ```$ docker-compose up```
4. Log in to the destination node and run backup-and-restore script.

# Usage
```# ./backup_and_restore -h         
Usage of ./backup_and_restore:
  -database string
        Influxdb database name to backup and restore (default "stress")
  -database-directory string
        Directory to store database (default "/tmp/stress")
  -firstrun
        Use this flag to execute a first time DB import.
  -influxdb-destination string
        Influxdb destination where to store the database (default "influxdb-destination:8088")
  -influxdb-source string
        Influxdb source where to query and get original database (default "influxdb-source:8088")
  -since int
        Create incremental backup after specified timestamp. Int values. Defaults to -1 hour (default -1)
```
# Examples
Test database created with [influx-stress](https://github.com/influxdata/influx-stress)

** All backup and restore commands must be run FROM the InfluxDB destination node!!! **

Initial backup (Running on influxdb-destination node)

```# ./backup_and_restore -firstrun```
