package graph

import (
	"slices"
	"testing"
)

func TestNew(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
		{Name: "backend", Deps: []string{"frontend", "api", "db"}},
	})

	if g.Len() != 4 {
		t.Fatalf("len: got %d, want 4", g.Len())
	}

	api := g.Node("api")
	if api == nil {
		t.Fatal("api not found")
	}
	if len(api.Deps) != 1 || api.Deps[0] != "db" {
		t.Fatalf("api.Deps: %v", api.Deps)
	}
	if !slices.Contains(api.DepsBy, "frontend") {
		t.Fatalf("api.DepsBy missing frontend: %v", api.DepsBy)
	}
	if !slices.Contains(api.DepsBy, "backend") {
		t.Fatalf("api.DepsBy missing backend: %v", api.DepsBy)
	}

	db := g.Node("db")
	if db == nil {
		t.Fatal("db not found")
	}
	if !slices.Contains(db.DepsBy, "api") {
		t.Fatalf("db.DepsBy missing api: %v", db.DepsBy)
	}
	if !slices.Contains(db.DepsBy, "backend") {
		t.Fatalf("db.DepsBy missing backend: %v", db.DepsBy)
	}

	frontend := g.Node("frontend")
	if frontend == nil {
		t.Fatal("frontend not found")
	}
	if !slices.Contains(frontend.DepsBy, "backend") {
		t.Fatalf("frontend.DepsBy missing backend: %v", frontend.DepsBy)
	}

	backend := g.Node("backend")
	if backend == nil {
		t.Fatal("backend not found")
	}
	if len(backend.DepsBy) != 0 {
		t.Fatalf("backend.DepsBy: %v (expected empty)", backend.DepsBy)
	}
}

func TestWalkDependents(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
	})

	var visited []string
	g.WalkDependents("db", func(name string) bool {
		visited = append(visited, name)
		return true
	})

	if len(visited) != 2 {
		t.Fatalf("visited: got %d, want 2: %v", len(visited), visited)
	}
	if visited[0] != "api" || visited[1] != "frontend" {
		t.Fatalf("visited order: %v", visited)
	}
}

func TestWalkDependentsMultiple(t *testing.T) {
	// db → api1, api2 → frontend
	g := New([]Edge{
		{Name: "db"},
		{Name: "api1", Deps: []string{"db"}},
		{Name: "api2", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api1", "api2"}},
	})

	var visited []string
	g.WalkDependents("db", func(name string) bool {
		visited = append(visited, name)
		return true
	})

	if len(visited) != 3 {
		t.Fatalf("visited: got %d, want 3: %v", len(visited), visited)
	}
	for _, want := range []string{"api1", "api2", "frontend"} {
		found := slices.Contains(visited, want)
		if !found {
			t.Fatalf("missing %s in %v", want, visited)
		}
	}
}

func TestWalkDependentsDiamond(t *testing.T) {
	//        ┌──────────┐
	//        │    db    │
	//        └─┬───┬───┬┘
	//          │   │   │
	//    ┌─────┘   │   └─────┐
	//    │     ┌───┴───┐     │
	//    │     │  api  │     │
	//    │     └───┬───┘     │
	//    │         │         │
	//    │   ┌─────┴─────┐   │
	//    │   │ frontend  │   │
	//    │   └─────┬─────┘   │
	//    │         │         │
	//    └─────────┼─────────┘
	//              │
	//        ┌─────┴─────┐
	//        │  backend  │
	//        └───────────┘
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
		{Name: "backend", Deps: []string{"frontend", "api", "db"}},
	})

	var visited []string
	g.WalkDependents("db", func(name string) bool {
		visited = append(visited, name)
		return true
	})

	if len(visited) != 3 {
		t.Fatalf("visited: got %d, want 3: %v", len(visited), visited)
	}
	for _, want := range []string{"api", "frontend", "backend"} {
		if !slices.Contains(visited, want) {
			t.Fatalf("missing %s in %v", want, visited)
		}
	}
	// api must appear before frontend (api's dependent)
	apiIdx := slices.Index(visited, "api")
	frontendIdx := slices.Index(visited, "frontend")
	if apiIdx >= frontendIdx {
		t.Fatalf("api (%d) must appear before frontend (%d): %v", apiIdx, frontendIdx, visited)
	}
}

func TestWalkDependentsStop(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
	})

	var visited []string
	g.WalkDependents("db", func(name string) bool {
		visited = append(visited, name)
		return name != "api" // stop after api
	})

	if len(visited) != 1 || visited[0] != "api" {
		t.Fatalf("visited: %v", visited)
	}
}

func TestNodeNotFound(t *testing.T) {
	g := New([]Edge{{Name: "a"}})
	if n := g.Node("missing"); n != nil {
		t.Fatal("expected nil for missing node")
	}
}

func TestStopLevelsChain(t *testing.T) {
	// db → api → frontend
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
	})

	levels := g.StopLevels()
	if len(levels) != 3 {
		t.Fatalf("got %d levels, want 3: %v", len(levels), levels)
	}
	// frontend has no dependents → level 0
	if !slices.Equal(levels[0], []string{"frontend"}) {
		t.Fatalf("level 0: %v", levels[0])
	}
	// api → level 1
	if !slices.Equal(levels[1], []string{"api"}) {
		t.Fatalf("level 1: %v", levels[1])
	}
	// db → level 2
	if !slices.Equal(levels[2], []string{"db"}) {
		t.Fatalf("level 2: %v", levels[2])
	}
}

func TestStopLevelsDiamond(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
		{Name: "backend", Deps: []string{"frontend", "api", "db"}},
	})

	levels := g.StopLevels()
	// backend has no dependents → level 0
	if !slices.Contains(levels[0], "backend") {
		t.Fatalf("level 0 missing backend: %v", levels[0])
	}
	if len(levels) != 4 {
		t.Fatalf("got %d levels, want 4: %v", len(levels), levels)
	}
	// Level 0: backend (nothing depends on it)
	// Level 1: frontend (only backend depended on it)
	// Level 2: api (frontend and backend depended on it)
	// Level 3: db (api and backend depended on it)
	if !slices.Contains(levels[0], "backend") {
		t.Fatalf("level 0 missing backend: %v", levels[0])
	}
	if !slices.Contains(levels[len(levels)-1], "db") {
		t.Fatalf("last level missing db: %v", levels[len(levels)-1])
	}
}

func TestStopLevelsEmpty(t *testing.T) {
	g := New(nil)
	levels := g.StopLevels()
	if len(levels) != 0 {
		t.Fatalf("expected empty, got %v", levels)
	}
}

func TestStartLevelsChain(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
	})

	levels := g.StartLevels()
	if len(levels) != 3 {
		t.Fatalf("got %d levels, want 3: %v", len(levels), levels)
	}
	// db has no deps → level 0
	if !slices.Equal(levels[0], []string{"db"}) {
		t.Fatalf("level 0: %v", levels[0])
	}
	// api → level 1
	if !slices.Equal(levels[1], []string{"api"}) {
		t.Fatalf("level 1: %v", levels[1])
	}
	// frontend → level 2
	if !slices.Equal(levels[2], []string{"frontend"}) {
		t.Fatalf("level 2: %v", levels[2])
	}
}

func TestStartLevelsDiamond(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
		{Name: "backend", Deps: []string{"frontend", "api", "db"}},
	})

	levels := g.StartLevels()
	if len(levels) != 4 {
		t.Fatalf("got %d levels, want 4: %v", len(levels), levels)
	}
	// db has no deps → level 0
	if !slices.Contains(levels[0], "db") {
		t.Fatalf("level 0 missing db: %v", levels[0])
	}
	// backend needs 3 things → must be last
	if !slices.Contains(levels[len(levels)-1], "backend") {
		t.Fatalf("last level missing backend: %v", levels[len(levels)-1])
	}
}

func TestLevelsCoverAllNodes(t *testing.T) {
	g := New([]Edge{
		{Name: "db"},
		{Name: "api", Deps: []string{"db"}},
		{Name: "frontend", Deps: []string{"api"}},
		{Name: "backend", Deps: []string{"frontend", "api", "db"}},
	})

	for _, levels := range [][][]string{g.StartLevels(), g.StopLevels()} {
		count := 0
		for _, level := range levels {
			count += len(level)
		}
		if count != g.Len() {
			t.Fatalf("levels cover %d of %d nodes", count, g.Len())
		}
	}
}
