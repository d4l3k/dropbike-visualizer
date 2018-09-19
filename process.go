package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	mapbox "github.com/ryankurte/go-mapbox/lib"
	"github.com/ryankurte/go-mapbox/lib/base"
	"github.com/ryankurte/go-mapbox/lib/directions"
	polyline "github.com/twpayne/go-polyline"
)

const filePath = "data/bikes-*.json.gz"

type Bike struct {
	Plate     string  `json:"plate"`
	Region    string  `json:"region"`
	HavenID   string  `json:"haven_id"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	log.SetOutput(os.Stderr)

	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

type bikeMeta struct {
	Bike

	LastMoved time.Time
}

var bikeHistory = map[string]*bikeMeta{}
var trips []Trip

type Point struct {
	Latitude, Longitude float64
}

type Trip struct {
	Plate              string
	Start, End         Point
	StartTime, EndTime time.Time

	Directions directions.DirectionResponse
	Coords     [][]float64
}

func processFile(file string) error {
	timestamp := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(file), ".json.gz"), "bikes-")
	t, err := time.Parse("2006-01-02T15:04:05-07:00", timestamp)
	if err != nil {
		return err
	}
	_ = t
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	var bikes []Bike
	if err := json.NewDecoder(gzr).Decode(&bikes); err != nil {
		return errors.Wrapf(err, "decoding %q", file)
	}

	for _, b := range bikes {
		bm, ok := bikeHistory[b.Plate]
		if !ok {
			bm = &bikeMeta{
				Bike:      b,
				LastMoved: t,
			}
			bikeHistory[b.Plate] = bm
		}
		cur := geo.NewPoint(b.Latitude, b.Longitude)
		old := geo.NewPoint(bm.Latitude, bm.Longitude)
		dist := old.GreatCircleDistance(cur)
		if dist < 0.100 { // 100 meters
			continue
		}
		geometry := directions.GeometryPolyline
		overview := directions.OverviewFull
		directions, err := mbox.Directions.GetDirections([]base.Location{
			{Latitude: bm.Latitude, Longitude: bm.Longitude},
			{Latitude: b.Latitude, Longitude: b.Longitude},
		}, directions.RoutingCycling, &directions.RequestOpts{
			Geometries: &geometry,
			Overview:   &overview,
		})
		if err != nil {
			return err
		}

		coords, _, err := polyline.DecodeCoords([]byte(directions.Routes[0].Geometry))
		if err != nil {
			return err
		}

		trips = append(trips, Trip{
			StartTime:  bm.LastMoved,
			EndTime:    t,
			Start:      Point{Latitude: bm.Latitude, Longitude: bm.Longitude},
			End:        Point{Latitude: b.Latitude, Longitude: b.Longitude},
			Plate:      b.Plate,
			Directions: *directions,
			Coords:     coords,
		})
		bm.Bike = b
		bm.LastMoved = t
	}

	return nil
}

var mbox *mapbox.Mapbox

func run() error {
	var err error
	mbox, err = mapbox.NewMapbox("pk.eyJ1IjoiZDRsM2siLCJhIjoiY2ptOGs3c3NwMHE2YzNxa2Q5bTc1cWV2cyJ9.AiwpzmG2c5SOIpoCeJJ20w")
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filePath)
	if err != nil {
		return err
	}
	for i, f := range files {
		if i%10 == 0 {
			log.Printf("processed %d / %d files", i, len(files))
		}
		if err := processFile(f); err != nil {
			log.Printf("bad file %q: %+v", f, err)
		}
	}
	data, err := json.Marshal(trips)
	if err != nil {
		return err
	}
	fmt.Printf("%s", data)
	return nil
}
