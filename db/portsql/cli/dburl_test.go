package cli

import (
	"testing"
)

func TestParseDBURL_Postgres(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantErr  bool
		dialect  string
		host     string
		port     string
		user     string
		password string
		database string
		query    string
	}{
		{
			name:     "basic postgres URL",
			url:      "postgres://localhost/mydb",
			dialect:  "postgres",
			host:     "localhost",
			database: "mydb",
		},
		{
			name:     "postgres URL with port",
			url:      "postgres://localhost:5432/mydb",
			dialect:  "postgres",
			host:     "localhost",
			port:     "5432",
			database: "mydb",
		},
		{
			name:     "postgres URL with user",
			url:      "postgres://user@localhost/mydb",
			dialect:  "postgres",
			host:     "localhost",
			user:     "user",
			database: "mydb",
		},
		{
			name:     "postgres URL with user and password",
			url:      "postgres://user:pass@localhost/mydb",
			dialect:  "postgres",
			host:     "localhost",
			user:     "user",
			password: "pass",
			database: "mydb",
		},
		{
			name:     "postgres URL with query params",
			url:      "postgres://localhost/mydb?sslmode=disable",
			dialect:  "postgres",
			host:     "localhost",
			database: "mydb",
			query:    "sslmode=disable",
		},
		{
			name:     "full postgres URL",
			url:      "postgres://user:pass@localhost:5432/mydb?sslmode=disable&connect_timeout=10",
			dialect:  "postgres",
			host:     "localhost",
			port:     "5432",
			user:     "user",
			password: "pass",
			database: "mydb",
			query:    "sslmode=disable&connect_timeout=10",
		},
		{
			name:     "postgresql:// scheme",
			url:      "postgresql://localhost/mydb",
			dialect:  "postgres",
			host:     "localhost",
			database: "mydb",
		},
		{
			name:     "127.0.0.1 host",
			url:      "postgres://127.0.0.1/mydb",
			dialect:  "postgres",
			host:     "127.0.0.1",
			database: "mydb",
		},
		{
			name:     "IPv6 localhost",
			url:      "postgres://[::1]/mydb",
			dialect:  "postgres",
			host:     "::1",
			database: "mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Dialect != tt.dialect {
				t.Errorf("Dialect = %q, want %q", parsed.Dialect, tt.dialect)
			}
			if parsed.Host != tt.host {
				t.Errorf("Host = %q, want %q", parsed.Host, tt.host)
			}
			if parsed.Port != tt.port {
				t.Errorf("Port = %q, want %q", parsed.Port, tt.port)
			}
			if parsed.User != tt.user {
				t.Errorf("User = %q, want %q", parsed.User, tt.user)
			}
			if parsed.Password != tt.password {
				t.Errorf("Password = %q, want %q", parsed.Password, tt.password)
			}
			if parsed.Database != tt.database {
				t.Errorf("Database = %q, want %q", parsed.Database, tt.database)
			}
			if parsed.Query != tt.query {
				t.Errorf("Query = %q, want %q", parsed.Query, tt.query)
			}
		})
	}
}

func TestParseDBURL_MySQL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		dialect  string
		host     string
		port     string
		user     string
		password string
		database string
		query    string
	}{
		{
			name:     "basic mysql URL",
			url:      "mysql://localhost/mydb",
			dialect:  "mysql",
			host:     "localhost",
			database: "mydb",
		},
		{
			name:     "mysql URL with port",
			url:      "mysql://localhost:3306/mydb",
			dialect:  "mysql",
			host:     "localhost",
			port:     "3306",
			database: "mydb",
		},
		{
			name:     "mysql URL with user and password",
			url:      "mysql://root:password@localhost:3306/mydb",
			dialect:  "mysql",
			host:     "localhost",
			port:     "3306",
			user:     "root",
			password: "password",
			database: "mydb",
		},
		{
			name:     "mysql URL with query params",
			url:      "mysql://localhost/mydb?parseTime=true&multiStatements=true",
			dialect:  "mysql",
			host:     "localhost",
			database: "mydb",
			query:    "parseTime=true&multiStatements=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Dialect != tt.dialect {
				t.Errorf("Dialect = %q, want %q", parsed.Dialect, tt.dialect)
			}
			if parsed.Host != tt.host {
				t.Errorf("Host = %q, want %q", parsed.Host, tt.host)
			}
			if parsed.Port != tt.port {
				t.Errorf("Port = %q, want %q", parsed.Port, tt.port)
			}
			if parsed.User != tt.user {
				t.Errorf("User = %q, want %q", parsed.User, tt.user)
			}
			if parsed.Password != tt.password {
				t.Errorf("Password = %q, want %q", parsed.Password, tt.password)
			}
			if parsed.Database != tt.database {
				t.Errorf("Database = %q, want %q", parsed.Database, tt.database)
			}
			if parsed.Query != tt.query {
				t.Errorf("Query = %q, want %q", parsed.Query, tt.query)
			}
		})
	}
}

func TestParseDBURL_SQLite(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		database string
		query    string
	}{
		{
			name:     "sqlite with relative path",
			url:      "sqlite://mydb.db",
			database: "mydb.db",
		},
		{
			name:     "sqlite with absolute path",
			url:      "sqlite:///var/data/mydb.db",
			database: "/var/data/mydb.db",
		},
		{
			name:     "sqlite simple format",
			url:      "sqlite:mydb.db",
			database: "mydb.db",
		},
		{
			name:     "sqlite with query params",
			url:      "sqlite://mydb.db?mode=memory",
			database: "mydb.db",
			query:    "mode=memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Dialect != "sqlite" {
				t.Errorf("Dialect = %q, want sqlite", parsed.Dialect)
			}
			if parsed.Database != tt.database {
				t.Errorf("Database = %q, want %q", parsed.Database, tt.database)
			}
			if parsed.Query != tt.query {
				t.Errorf("Query = %q, want %q", parsed.Query, tt.query)
			}
		})
	}
}

func TestParseDBURL_Errors(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "empty URL",
			url:  "",
		},
		{
			name: "unsupported scheme",
			url:  "mongodb://localhost/mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDBURL(tt.url)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParsedDBURL_WithDatabase(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		newDB    string
		expected string
	}{
		{
			name:     "postgres simple",
			url:      "postgres://localhost/olddb",
			newDB:    "newdb",
			expected: "postgres://localhost/newdb",
		},
		{
			name:     "postgres with all options",
			url:      "postgres://user:pass@localhost:5432/olddb?sslmode=disable",
			newDB:    "newdb",
			expected: "postgres://user:pass@localhost:5432/newdb?sslmode=disable",
		},
		{
			name:     "mysql simple",
			url:      "mysql://localhost/olddb",
			newDB:    "newdb",
			expected: "mysql://localhost/newdb",
		},
		{
			name:     "mysql with options",
			url:      "mysql://root:pass@localhost:3306/olddb?parseTime=true",
			newDB:    "newdb",
			expected: "mysql://root:pass@localhost:3306/newdb?parseTime=true",
		},
		{
			name:     "sqlite",
			url:      "sqlite://old.db",
			newDB:    "newdb",
			expected: "sqlite://newdb.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			result := parsed.WithDatabase(tt.newDB)
			if result != tt.expected {
				t.Errorf("WithDatabase() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParsedDBURL_WithDatabase_RoundTrip(t *testing.T) {
	// Test that WithDatabase(WithDatabase(url, a), b) == WithDatabase(url, b)
	urls := []string{
		"postgres://user:pass@localhost:5432/original?sslmode=disable",
		"mysql://root@localhost:3306/original",
		"sqlite://original.db",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			parsed, err := ParseDBURL(url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			// First rewrite
			url1 := parsed.WithDatabase("intermediate")
			parsed1, err := ParseDBURL(url1)
			if err != nil {
				t.Fatalf("failed to parse first rewrite: %v", err)
			}

			// Second rewrite
			url2 := parsed1.WithDatabase("final")

			// Direct rewrite
			urlDirect := parsed.WithDatabase("final")

			if url2 != urlDirect {
				t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", url2, urlDirect)
			}
		})
	}
}

func TestParsedDBURL_IsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "localhost",
			url:      "postgres://localhost/mydb",
			expected: true,
		},
		{
			name:     "127.0.0.1",
			url:      "postgres://127.0.0.1/mydb",
			expected: true,
		},
		{
			name:     "::1 IPv6",
			url:      "postgres://[::1]/mydb",
			expected: true,
		},
		{
			name:     "sqlite always local",
			url:      "sqlite://mydb.db",
			expected: true,
		},
		{
			name:     "remote host",
			url:      "postgres://db.example.com/mydb",
			expected: false,
		},
		{
			name:     "192.168.1.1 is not localhost",
			url:      "postgres://192.168.1.1/mydb",
			expected: false,
		},
		{
			name:     "10.0.0.1 is not localhost",
			url:      "mysql://10.0.0.1/mydb",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			if parsed.IsLocalhost() != tt.expected {
				t.Errorf("IsLocalhost() = %v, want %v", parsed.IsLocalhost(), tt.expected)
			}
		})
	}
}

func TestParsedDBURL_MaintenanceURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "postgres",
			url:      "postgres://user:pass@localhost:5432/mydb",
			expected: "postgres://user:pass@localhost:5432/postgres",
		},
		{
			name:     "mysql",
			url:      "mysql://root@localhost:3306/mydb",
			expected: "mysql://root@localhost:3306/",
		},
		{
			name:     "sqlite",
			url:      "sqlite://mydb.db",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDBURL(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			result := parsed.MaintenanceURL()
			if result != tt.expected {
				t.Errorf("MaintenanceURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDeriveDBNames(t *testing.T) {
	tests := []struct {
		name             string
		baseName         string
		devNameOverride  string
		testNameOverride string
		projectFolder    string
		wantDev          string
		wantTest         string
	}{
		{
			name:          "default derivation from folder",
			projectFolder: "myproject",
			wantDev:       "myproject",
			wantTest:      "myproject_test",
		},
		{
			name:          "base name overrides folder",
			baseName:      "custom",
			projectFolder: "myproject",
			wantDev:       "custom",
			wantTest:      "custom_test",
		},
		{
			name:            "explicit dev name",
			devNameOverride: "mydev",
			projectFolder:   "myproject",
			wantDev:         "mydev",
			wantTest:        "myproject_test",
		},
		{
			name:             "explicit test name",
			testNameOverride: "mytest",
			projectFolder:    "myproject",
			wantDev:          "myproject",
			wantTest:         "mytest",
		},
		{
			name:             "both explicit",
			devNameOverride:  "mydev",
			testNameOverride: "mytest",
			projectFolder:    "myproject",
			wantDev:          "mydev",
			wantTest:         "mytest",
		},
		{
			name:             "explicit overrides base",
			baseName:         "base",
			devNameOverride:  "mydev",
			testNameOverride: "mytest",
			projectFolder:    "myproject",
			wantDev:          "mydev",
			wantTest:         "mytest",
		},
		{
			name:          "hyphen to underscore",
			projectFolder: "my-project",
			wantDev:       "my_project",
			wantTest:      "my_project_test",
		},
		{
			name:          "removes invalid characters",
			projectFolder: "my.project@name",
			wantDev:       "myprojectname",
			wantTest:      "myprojectname_test",
		},
		{
			name:          "preserves underscores",
			projectFolder: "my_project_name",
			wantDev:       "my_project_name",
			wantTest:      "my_project_name_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, test := DeriveDBNames(tt.baseName, tt.devNameOverride, tt.testNameOverride, tt.projectFolder)
			if dev != tt.wantDev {
				t.Errorf("dev = %q, want %q", dev, tt.wantDev)
			}
			if test != tt.wantTest {
				t.Errorf("test = %q, want %q", test, tt.wantTest)
			}
		})
	}
}

func TestValidateDBName(t *testing.T) {
	tests := []struct {
		name    string
		dbName  string
		wantErr bool
	}{
		{
			name:    "valid simple name",
			dbName:  "mydb",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			dbName:  "my_db_name",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			dbName:  "mydb123",
			wantErr: false,
		},
		{
			name:    "valid starting with underscore",
			dbName:  "_mydb",
			wantErr: false,
		},
		{
			name:    "empty name",
			dbName:  "",
			wantErr: true,
		},
		{
			name:    "starts with number",
			dbName:  "1mydb",
			wantErr: true,
		},
		{
			name:    "contains hyphen",
			dbName:  "my-db",
			wantErr: true,
		},
		{
			name:    "contains dot",
			dbName:  "my.db",
			wantErr: true,
		},
		{
			name:    "contains space",
			dbName:  "my db",
			wantErr: true,
		},
		{
			name:    "too long",
			dbName:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 65 chars
			wantErr: true,
		},
		{
			name:    "max length ok",
			dbName:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 63 chars
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDBName(tt.dbName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDBName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "mydb",
			expected: `"mydb"`,
		},
		{
			name:     "name with double quote",
			input:    `my"db`,
			expected: `"my""db"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quoteIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("quoteIdentifier() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestQuoteIdentifierMySQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "mydb",
			expected: "`mydb`",
		},
		{
			name:     "name with backtick",
			input:    "my`db",
			expected: "`my``db`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quoteIdentifierMySQL(tt.input)
			if result != tt.expected {
				t.Errorf("quoteIdentifierMySQL() = %q, want %q", result, tt.expected)
			}
		})
	}
}
