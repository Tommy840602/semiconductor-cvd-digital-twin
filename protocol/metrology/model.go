// Package metrology implements a synthetic demonstration model, not a KLA instrument specification.
package metrology

import (
	"crypto/sha256"
	"cvd/protocol/contract"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
)

const Station = "KLA-SIM-01"
const Model = "synthetic-kla-v1"

type Source struct {
	EquipmentID     string    `json:"equipmentId"`
	WaferID         string    `json:"waferId"`
	LotID           string    `json:"lotId"`
	Slot            int       `json:"slot"`
	PM              string    `json:"pm"`
	Purpose         string    `json:"purpose"`
	ProcessRunID    string    `json:"processRunId"`
	ExecutionID     string    `json:"executionId,omitempty"`
	RecipeVersion   string    `json:"recipeVersion"`
	Thickness       float64   `json:"thicknessNm"`
	Samples         []float64 `json:"sampleThicknessNm"`
	Target          float64   `json:"targetThicknessNm"`
	Tolerance       float64   `json:"thicknessTolerancePct"`
	UniformityLimit float64   `json:"uniformityLimitPct"`
}
type Point struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Thickness float64 `json:"thicknessNm"`
}
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Defect struct {
	ID       string  `json:"defectId"`
	Type     string  `json:"type"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Size     float64 `json:"sizeUm"`
	Severity string  `json:"severity"`
	Path     []XY    `json:"path,omitempty"`
}
type Result struct {
	Model            string   `json:"modelVersion"`
	Seed             string   `json:"seed"`
	Synthetic        bool     `json:"synthetic"`
	Source           Source   `json:"source"`
	Diameter         float64  `json:"diameterMm"`
	Edge             float64  `json:"edgeExclusionMm"`
	Coordinates      string   `json:"coordinates"`
	Points           []Point  `json:"points"`
	Defects          []Defect `json:"defects"`
	Mean             float64  `json:"meanNm"`
	Min              float64  `json:"minNm"`
	Max              float64  `json:"maxNm"`
	Uniformity       *float64 `json:"uniformityPct"`
	ThicknessQuality string   `json:"thicknessQuality"`
	DefectQuality    string   `json:"defectQuality"`
	Quality          string   `json:"quality"`
	DefectCountLimit int      `json:"defectCountLimit"`
	DefectSizeLimit  float64  `json:"defectSizeLimitUm"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (s Source) Validate() error {
	if !contract.ValidEquipment(s.EquipmentID) || s.WaferID == "" || s.LotID == "" || s.ProcessRunID == "" || s.RecipeVersion == "" || s.Slot < 1 || s.Slot > 25 || len(s.Samples) != 5 {
		return fmt.Errorf("missing source")
	}
	if s.PM != "PM1" && s.PM != "PM2" && s.PM != "PM3" && s.PM != "PM4" {
		return fmt.Errorf("invalid PM")
	}
	for _, v := range append(append([]float64{}, s.Samples...), s.Thickness, s.Target, s.Tolerance, s.UniformityLimit) {
		if !finite(v) || v < 0 {
			return fmt.Errorf("invalid numeric source")
		}
	}
	if s.Target <= 0 {
		return fmt.Errorf("missing target")
	}
	return nil
}
func Generate(s Source) (Result, error) {
	r := Result{}
	if e := s.Validate(); e != nil {
		return r, e
	}
	raw, _ := json.Marshal(s)
	seed := sha256.Sum256(append([]byte(Model), raw...))
	rng := rand.New(rand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:16])))
	r = Result{Model: Model, Seed: fmt.Sprintf("%x", seed), Synthetic: true, Source: s, Diameter: 300, Edge: 3, Coordinates: "center-x-right-y-up-notch-down-mm", Points: []Point{}, Defects: []Defect{}, Min: math.MaxFloat64, DefectCountLimit: 10, DefectSizeLimit: 1, ThicknessQuality: "pass", DefectQuality: "pass", Quality: "pass"}
	add := func(x, y float64) {
		v := math.Max(0, s.Thickness+(s.Samples[1]-s.Samples[2])*x/294+(s.Samples[3]-s.Samples[4])*y/294+(s.Samples[0]-s.Thickness)*(x*x+y*y)/(147*147)+s.Thickness*(rng.Float64()-.5)*.008)
		r.Points = append(r.Points, Point{x, y, v})
		r.Mean += v
		r.Min = math.Min(r.Min, v)
		r.Max = math.Max(r.Max, v)
	}
	add(0, 0)
	for ring := 1; ring <= 3; ring++ {
		for n := 0; n < 8*ring; n++ {
			a := 2 * math.Pi * float64(n) / float64(8*ring)
			add(float64(ring)*47*math.Cos(a), float64(ring)*47*math.Sin(a))
		}
	}
	r.Mean /= float64(len(r.Points))
	if r.Mean > 0 {
		u := (r.Max - r.Min) / 2 / r.Mean * 100
		r.Uniformity = &u
	}
	for _, p := range r.Points {
		if math.Abs(p.Thickness-s.Target)/s.Target*100 > s.Tolerance {
			r.ThicknessQuality = "review"
		}
	}
	if r.Uniformity == nil || *r.Uniformity > s.UniformityLimit {
		r.ThicknessQuality = "review"
	}
	count := rng.IntN(15)
	for i := 0; i < count; i++ {
		a := rng.Float64() * 2 * math.Pi
		radius := math.Sqrt(rng.Float64()) * 140
		size := .05 + rng.Float64()*1.1
		typ := []string{"particle", "scratch", "other"}[rng.IntN(3)]
		d := Defect{ID: fmt.Sprintf("D%04d", i+1), Type: typ, X: radius * math.Cos(a), Y: radius * math.Sin(a), Size: size, Severity: "minor"}
		if size > 1 {
			d.Severity = "major"
			r.DefectQuality = "review"
		}
		if typ == "scratch" {
			d.Path = []XY{{d.X, d.Y}, {d.X + 2, d.Y + 2}}
		}
		r.Defects = append(r.Defects, d)
	}
	if count > r.DefectCountLimit {
		r.DefectQuality = "review"
	}
	if r.ThicknessQuality == "review" || r.DefectQuality == "review" {
		r.Quality = "review"
	}
	return r, nil
}
func Validate(r Result) error {
	if err := r.Source.Validate(); err != nil {
		return err
	}
	if len(r.Seed) != 64 || r.Model != Model || !r.Synthetic || r.Diameter != 300 || r.Edge != 3 || r.Coordinates != "center-x-right-y-up-notch-down-mm" || len(r.Points) != 49 || r.Defects == nil || len(r.Defects) > 1000 {
		return fmt.Errorf("incomplete map")
	}
	for _, p := range r.Points {
		if !finite(p.X) || !finite(p.Y) || !finite(p.Thickness) || p.Thickness < 0 || math.Hypot(p.X, p.Y) > 147 {
			return fmt.Errorf("invalid point")
		}
	}
	ids := map[string]bool{}
	for _, d := range r.Defects {
		if d.ID == "" || ids[d.ID] || !finite(d.X) || !finite(d.Y) || !finite(d.Size) || d.Size < 0 || math.Hypot(d.X, d.Y) > 147 {
			return fmt.Errorf("invalid defect")
		}
		ids[d.ID] = true
		for _, p := range d.Path {
			if !finite(p.X) || !finite(p.Y) || math.Hypot(p.X, p.Y) > 147 {
				return fmt.Errorf("invalid scratch")
			}
		}
	}
	return nil
}
