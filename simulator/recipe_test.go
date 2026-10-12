package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func recipeCommand(values map[string]any) Command {
	return Command{CommandID: "recipe", Type: "configure", ExpiresAt: time.Now().Add(time.Hour), Parameters: values}
}
func TestCustomRecipeAndCheckpoint(t *testing.T) {
	e := NewEngine()
	steps := map[string]any{"preheat": float64(2), "stabilize": float64(3), "deposition": float64(7), "purge": float64(4)}
	r := e.Execute(recipeCommand(map[string]any{"stepSeconds": steps}), time.Now())
	if r.Status != "succeeded" {
		t.Fatal(r)
	}
	if e.ProcessSeconds != 16 {
		t.Fatal(e.ProcessSeconds)
	}
	start(e, 1)
	for e.Chambers[0].Process == nil {
		tick(t, e)
	}
	p := e.Chambers[0]
	counts := map[string]int{}
	// beginProcess is called at transport completion before the processing tick.
	if p.Process.Elapsed > 0 {
		counts[p.Process.Phase] = p.Process.Elapsed
	}
	for p.Process.Elapsed < 16 {
		e.stepProcess(p)
		counts[p.Process.Phase]++
	}
	if !reflect.DeepEqual(counts, map[string]int{"preheat": 2, "stabilize": 3, "deposition": 7, "purge": 4}) {
		t.Fatal(counts)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if *loaded.RecipeSteps != [4]int{2, 3, 7, 4} || loaded.Chambers[0].Process.StepSeconds != [4]int{2, 3, 7, 4} {
		t.Fatal("recipe not preserved")
	}
	c := recipeCommand(map[string]any{"stepSeconds": map[string]any{"preheat": 1.0, "stabilize": 1.0, "deposition": 2.0, "purge": 1.0}, "recipeVersion": "new-version"})
	c.CommandID = "busy"
	if e.Execute(c, time.Now()).Status != "succeeded" || e.ProcessSeconds != 5 || p.Process.StepSeconds != [4]int{2, 3, 7, 4} {
		t.Fatal("recipe update changed in-flight process")
	}
	next := &Chamber{ID: "PM2"}
	e.beginProcess(next, e.Lots[0].Wafers[1])
	if next.Process.RecipeVersion != "new-version" || next.Process.StepSeconds != [4]int{1, 1, 2, 1} {
		t.Fatal("new process did not use new recipe")
	}
}
func TestRecipeInvalidAtomic(t *testing.T) {
	for _, bad := range []any{0.0, -1.0, 1.5, 3601.0, "2", nil} {
		e := NewEngine()
		r := e.Execute(recipeCommand(map[string]any{"stepSeconds": map[string]any{"preheat": bad, "stabilize": 1.0, "deposition": 1.0, "purge": 1.0}}), time.Now())
		if r.Status != "rejected" || e.ProcessSeconds != 30 || e.RecipeSteps != nil {
			t.Fatal(bad, r)
		}
	}
	for _, v := range []map[string]any{{"preheat": 1.0}, {"preheat": 3600.0, "stabilize": 1.0, "deposition": 1.0, "purge": 1.0}} {
		e := NewEngine()
		if e.Execute(recipeCommand(map[string]any{"stepSeconds": v}), time.Now()).Status != "rejected" {
			t.Fatal(v)
		}
	}
}
