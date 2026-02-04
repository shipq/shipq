package dburl

import (
	"errors"
	"testing"
)

func TestInferDialectFromDBUrl(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr error
	}{
		{
			name: "postgres URL",
			url:  "postgres://postgres@localhost:5432/mydb",
			want: DialectPostgres,
		},
		{
			name: "postgresql URL",
			url:  "postgresql://user@localhost:5432/mydb",
			want: DialectPostgres,
		},
		{
			name: "mysql URL",
			url:  "mysql://root@localhost:3306/mydb",
			want: DialectMySQL,
		},
		{
			name: "sqlite URL",
			url:  "sqlite:///path/to/db.sqlite",
			want: DialectSQLite,
		},
		{
			name: "sqlite3 URL",
			url:  "sqlite3:///path/to/db.sqlite",
			want: DialectSQLite,
		},
		{
			name:    "unknown scheme",
			url:     "mongodb://localhost/db",
			wantErr: ErrUnknownDialect,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: ErrUnknownDialect,
		},
		{
			name: "uppercase scheme",
			url:  "POSTGRES://localhost/db",
			want: DialectPostgres,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InferDialectFromDBUrl(tt.url)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "localhost",
			url:  "postgres://user@localhost:5432/db",
			want: true,
		},
		{
			name: "127.0.0.1",
			url:  "postgres://user@127.0.0.1:5432/db",
			want: true,
		},
		{
			name: "::1 IPv6 localhost",
			url:  "postgres://user@[::1]:5432/db",
			want: true,
		},
		{
			name: "remote host",
			url:  "postgres://user@db.example.com:5432/db",
			want: false,
		},
		{
			name: "remote IP",
			url:  "postgres://user@192.168.1.100:5432/db",
			want: false,
		},
		{
			name: "sqlite is always local",
			url:  "sqlite:///path/to/db.sqlite",
			want: true,
		},
		{
			name: "invalid URL",
			url:  "://invalid",
			want: false,
		},
		{
			name: "LOCALHOST uppercase",
			url:  "postgres://user@LOCALHOST:5432/db",
			want: true,
		},
		{
			name: "mysql localhost",
			url:  "mysql://root@localhost:3306/db",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalhost(tt.url)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPostgresURL(t *testing.T) {
	tests := []struct {
		name   string
		dbname string
		user   string
		host   string
		port   int
		want   string
	}{
		{
			name:   "standard postgres URL",
			dbname: "mydb",
			user:   "postgres",
			host:   "localhost",
			port:   5432,
			want:   "postgres://postgres@localhost:5432/mydb",
		},
		{
			name:   "custom port",
			dbname: "testdb",
			user:   "admin",
			host:   "127.0.0.1",
			port:   5433,
			want:   "postgres://admin@127.0.0.1:5433/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPostgresURL(tt.dbname, tt.user, tt.host, tt.port)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMySQLURL(t *testing.T) {
	tests := []struct {
		name   string
		dbname string
		user   string
		host   string
		port   int
		want   string
	}{
		{
			name:   "standard mysql URL",
			dbname: "mydb",
			user:   "root",
			host:   "localhost",
			port:   3306,
			want:   "mysql://root@localhost:3306/mydb",
		},
		{
			name:   "custom port",
			dbname: "testdb",
			user:   "admin",
			host:   "127.0.0.1",
			port:   3307,
			want:   "mysql://admin@127.0.0.1:3307/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMySQLURL(tt.dbname, tt.user, tt.host, tt.port)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSQLiteURL(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		want     string
	}{
		{
			name:     "absolute path",
			filepath: "/path/to/db.sqlite",
			want:     "sqlite:///path/to/db.sqlite",
		},
		{
			name:     "relative path",
			filepath: "./data/db.sqlite",
			want:     "sqlite:./data/db.sqlite",
		},
		{
			name:     "relative path without dot",
			filepath: "data/db.sqlite",
			want:     "sqlite:data/db.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSQLiteURL(tt.filepath)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDatabaseName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "postgres URL",
			url:  "postgres://user@localhost:5432/mydb",
			want: "mydb",
		},
		{
			name: "mysql URL",
			url:  "mysql://root@localhost:3306/testdb",
			want: "testdb",
		},
		{
			name: "URL without database",
			url:  "postgres://user@localhost:5432",
			want: "",
		},
		{
			name: "URL with empty path",
			url:  "postgres://user@localhost:5432/",
			want: "",
		},
		{
			name: "invalid URL",
			url:  "://invalid",
			want: "",
		},
		{
			name: "sqlite URL",
			url:  "sqlite:///path/to/db.sqlite",
			want: "path/to/db.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDatabaseName(tt.url)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithDatabaseName(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		dbname  string
		want    string
		wantErr bool
	}{
		{
			name:   "postgres URL",
			url:    "postgres://user@localhost:5432/olddb",
			dbname: "newdb",
			want:   "postgres://user@localhost:5432/newdb",
		},
		{
			name:   "mysql URL",
			url:    "mysql://root@localhost:3306/olddb",
			dbname: "newdb",
			want:   "mysql://root@localhost:3306/newdb",
		},
		{
			name:   "URL without database",
			url:    "postgres://user@localhost:5432",
			dbname: "newdb",
			want:   "postgres://user@localhost:5432/newdb",
		},
		{
			name:    "invalid URL",
			url:     "://invalid",
			dbname:  "db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WithDatabaseName(tt.url, tt.dbname)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
