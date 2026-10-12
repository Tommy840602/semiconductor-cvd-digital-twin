package metrology

import (
	"math"
	"reflect"
	"testing"
)

func source() Source {
	return Source{EquipmentID: "TEL-CVDTIN-08A-01", WaferID: "w1", LotID: "l1", Slot: 1, PM: "PM1", ProcessRunID: "w1/1", RecipeVersion: "recipe1", Thickness: 10, Samples: []float64{10, 10.1, 9.9, 10.2, 9.8}, Target: 10, Tolerance: 10, UniformityLimit: 5}
}
func TestModel(t *testing.T) {
	s := source()
	a, e := Generate(s)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := Generate(s)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic")
	}
	if e = Validate(a); e != nil {
		t.Fatal(e)
	}
	sum := 0.
	for _, p := range a.Points {
		sum += p.Thickness
	}
	if math.Abs(sum/49-a.Mean) > 1e-9 {
		t.Fatal("mean")
	}
	s.Thickness = 20
	c, _ := Generate(s)
	if c.ThicknessQuality != "review" {
		t.Fatal("no thickness alarm")
	}
}
func TestRejectInvalid(t *testing.T) {
	s := source()
	s.Samples = nil
	if _, e := Generate(s); e == nil {
		t.Fatal("missing source accepted")
	}
	s = source()
	s.Thickness = math.NaN()
	if _, e := Generate(s); e == nil {
		t.Fatal("nan")
	}
	r, _ := Generate(source())
	r.Defects = make([]Defect, 1001)
	if Validate(r) == nil {
		t.Fatal("oversize")
	}
	r, _ = Generate(source())
	r.Points = r.Points[:48]
	if Validate(r) == nil {
		t.Fatal("partial")
	}
}
