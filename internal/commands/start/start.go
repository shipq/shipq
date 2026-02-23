package start

import (
	"fmt"
	"os"
)

// validServices is the authoritative list of services that "shipq start" supports.
var validServices = []string{
	"postgres",
	"mysql",
	"sqlite",
	"redis",
	"minio",
	"centrifugo",
	"server",
	"worker",
}

const startUsage = `Usage: shipq start <service>

Start a local dev service as a foreground process.

Services:
  postgres    Start a PostgreSQL server
  mysql       Start a MySQL server
  sqlite      Initialise the SQLite database file (no server required)
  redis       Start a Redis server
  minio       Start a MinIO S3-compatible object store
  centrifugo  Start Centrifugo (WebSocket hub)
  server      Run the application server  (go run ./cmd/server)
  worker      Run the background worker   (go run ./cmd/worker)

Each service runs in the foreground. Open a separate terminal tab for each
one you need, or use a process manager such as overmind / goreman.

Press Ctrl-C in any terminal to stop the corresponding service.
`

// StartCmd dispatches "shipq start <service>" to the correct starter function.
func StartCmd(service string) {
	switch service {
	case "postgres":
		StartPostgres()
	case "mysql":
		StartMySQL()
	case "sqlite":
		StartSQLite()
	case "redis":
		StartRedis()
	case "minio":
		StartMinio()
	case "centrifugo":
		StartCentrifugo()
	case "server":
		StartServer()
	case "worker":
		StartWorker()
	case "-h", "--help", "help":
		fmt.Print(startUsage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown service %q\n", service)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Valid services: postgres, mysql, sqlite, redis, minio, centrifugo, server, worker")
		fmt.Fprintln(os.Stderr, "Run 'shipq start --help' for usage.")
		os.Exit(1)
	}
}

// ValidServices returns a copy of the authoritative service name list.
// Useful for tests and help-text generation.
func ValidServices() []string {
	out := make([]string, len(validServices))
	copy(out, validServices)
	return out
}
