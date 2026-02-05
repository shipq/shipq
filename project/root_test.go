package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootFrom(t *testing.T) {
	t.Run("finds project root in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		root, err := FindProjectRootFrom(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("finds project root in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		subDir := filepath.Join(tmpDir, "sub", "deep")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		root, err := FindProjectRootFrom(subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("returns error when no go.mod found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := FindProjectRootFrom(tmpDir)
		if err != ErrNotInProject {
			t.Errorf("got error %v, want %v", err, ErrNotInProject)
		}
	})
}

func TestValidateProjectRoot(t *testing.T) {
	t.Run("valid project with both files", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)

		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		err := ValidateProjectRoot(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing go.mod", func(t *testing.T) {
		tmpDir := t.TempDir()
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)

		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		err := ValidateProjectRoot(tmpDir)
		if err != ErrNoGoMod {
			t.Errorf("got error %v, want %v", err, ErrNoGoMod)
		}
	})

	t.Run("missing shipq.ini", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)

		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		err := ValidateProjectRoot(tmpDir)
		if err != ErrNoShipqIni {
			t.Errorf("got error %v, want %v", err, ErrNoShipqIni)
		}
	})

	t.Run("missing both files", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := ValidateProjectRoot(tmpDir)
		if err != ErrNoGoMod {
			t.Errorf("got error %v, want %v (go.mod is checked first)", err, ErrNoGoMod)
		}
	})
}

func TestGetProjectName(t *testing.T) {
	t.Run("returns folder name", func(t *testing.T) {
		tests := []struct {
			path string
			want string
		}{
			{"/home/user/projects/myproject", "myproject"},
			{"/var/www/app", "app"},
			{"./relative/path/project", "project"},
		}

		for _, tt := range tests {
			got := GetProjectName(tt.path)
			if got != tt.want {
				t.Errorf("GetProjectName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		}
	})
}

func TestHasGoMod(t *testing.T) {
	t.Run("returns true when go.mod exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		if !HasGoMod(tmpDir) {
			t.Error("expected HasGoMod to return true")
		}
	})

	t.Run("returns false when go.mod doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		if HasGoMod(tmpDir) {
			t.Error("expected HasGoMod to return false")
		}
	})
}

func TestHasShipqIni(t *testing.T) {
	t.Run("returns true when shipq.ini exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		if !HasShipqIni(tmpDir) {
			t.Error("expected HasShipqIni to return true")
		}
	})

	t.Run("returns false when shipq.ini doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		if HasShipqIni(tmpDir) {
			t.Error("expected HasShipqIni to return false")
		}
	})
}

func TestFindShipqRootFrom(t *testing.T) {
	t.Run("finds shipq root in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		root, err := FindShipqRootFrom(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("finds shipq root in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		subDir := filepath.Join(tmpDir, "sub", "deep")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		root, err := FindShipqRootFrom(subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("returns error when no shipq.ini found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := FindShipqRootFrom(tmpDir)
		if err != ErrNoShipqIni {
			t.Errorf("got error %v, want %v", err, ErrNoShipqIni)
		}
	})
}

func TestFindProjectRootsFrom(t *testing.T) {
	t.Run("standard setup - both files in same directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)

		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		roots, err := FindProjectRootsFrom(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if roots.GoModRoot != tmpDir {
			t.Errorf("GoModRoot = %q, want %q", roots.GoModRoot, tmpDir)
		}
		if roots.ShipqRoot != tmpDir {
			t.Errorf("ShipqRoot = %q, want %q", roots.ShipqRoot, tmpDir)
		}
	})

	t.Run("monorepo setup - shipq.ini in subdirectory of go.mod", func(t *testing.T) {
		// Create monorepo structure:
		// tmpDir/
		//   go.mod
		//   services/
		//     myservice/
		//       shipq.ini
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		if err := os.WriteFile(goModPath, []byte("module github.com/company/monorepo\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		serviceDir := filepath.Join(tmpDir, "services", "myservice")
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			t.Fatalf("failed to create service directory: %v", err)
		}

		shipqIniPath := filepath.Join(serviceDir, ShipqIniFile)
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		roots, err := FindProjectRootsFrom(serviceDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if roots.GoModRoot != tmpDir {
			t.Errorf("GoModRoot = %q, want %q", roots.GoModRoot, tmpDir)
		}
		if roots.ShipqRoot != serviceDir {
			t.Errorf("ShipqRoot = %q, want %q", roots.ShipqRoot, serviceDir)
		}
	})

	t.Run("error when shipq.ini not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModPath := filepath.Join(tmpDir, GoModFile)
		if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		_, err := FindProjectRootsFrom(tmpDir)
		if err != ErrNoShipqIni {
			t.Errorf("got error %v, want %v", err, ErrNoShipqIni)
		}
	})

	t.Run("error when go.mod not found above shipq.ini", func(t *testing.T) {
		tmpDir := t.TempDir()
		shipqIniPath := filepath.Join(tmpDir, ShipqIniFile)
		if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}

		_, err := FindProjectRootsFrom(tmpDir)
		if err != ErrNotInProject {
			t.Errorf("got error %v, want %v", err, ErrNotInProject)
		}
	})
}
