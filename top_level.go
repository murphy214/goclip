package polyclip

import (
	pc "./polyclip"
	"bytes"
	"encoding/gob"
	//"fmt"
	m "github.com/murphy214/mercantile"
	"github.com/paulmach/go.geojson"
	"math"
	//"strings"
)

func ClipPolygon(subject [][][]float64, about [][][]float64, operation pc.Op) [][][][]float64 {
	return LintPolygons(
		Make(subject).Construct(
			operation,
			Make(about),
		),
	)
}

// tile clipping
func TileClip(about pc.Polygon, tile m.TileID) [][][][]float64 {
	bds := m.Bounds(tile)
	return LintPolygons(pc.Polygon{
		{pc.Point{bds.E, bds.N},
			pc.Point{bds.W, bds.N},
			pc.Point{bds.W, bds.S},
			pc.Point{bds.E, bds.S},
		},
	}.Construct(
		pc.INTERSECTION,
		about,
	))
}

func CreateBBs(minx, maxx, miny, maxy, zoom int) []m.TileID {
	tileids := make([]m.TileID, (maxx-minx+1)*(maxy-miny+1))
	pos := 0
	for currentx := minx; currentx <= maxx; currentx++ {
		for currenty := miny; currenty <= maxy; currenty++ {
			tileids[pos] = m.TileID{int64(currentx), int64(currenty), uint64(zoom)}
			pos++
		}
	}
	return tileids
}

type Output struct {
	TileID   m.TileID
	Features [][][][]float64
}

func PolygonClipNaive(feat [][][]float64, zoom int) map[m.TileID][][][][]float64 {
	// getting bbs as well as poly clip
	poly, bds := MakeBds(feat)
	// getting all four corners
	c1 := pc.Point{bds.E, bds.N}
	c3 := pc.Point{bds.W, bds.S}
	c1t := m.Tile(c1.X, c1.Y, zoom)
	c3t := m.Tile(c3.X, c3.Y, zoom)

	minx, maxx := int(math.Min(float64(c1t.X), float64(c3t.X))), int(math.Max(float64(c1t.X), float64(c3t.X)))
	miny, maxy := int(math.Min(float64(c1t.Y), float64(c3t.Y))), int(math.Max(float64(c1t.Y), float64(c3t.Y)))

	if minx == maxx && miny == maxy {
		return map[m.TileID][][][][]float64{m.TileID{int64(minx), int64(miny), uint64(zoom)}: [][][][]float64{feat}}
	}

	bbs := CreateBBs(minx, maxx, miny, maxy, zoom)

	c := make(chan Output)
	for _, bb := range bbs {
		go func(poly pc.Polygon, bb m.TileID, c chan Output) {
			c <- Output{bb, TileClip(poly, bb)}
		}(poly, bb, c)
	}

	totalmap := map[m.TileID][][][][]float64{}
	for range bbs {
		out := <-c
		if len(out.Features) > 0 {
			totalmap[out.TileID] = append(totalmap[out.TileID], out.Features...)
		}
	}

	return totalmap
}

// structure for finding overlapping values
func Overlapping1D(box1min float64, box1max float64, box2min float64, box2max float64) bool {
	if box1max >= box2min && box2max >= box1min {
		return true
	} else {
		return false
	}
	return false
}

// returns a boolval for whether or not the bb intersects
func Intersect(bdsref m.Extrema, bds m.Extrema) bool {
	if Overlapping1D(bdsref.W, bdsref.E, bds.W, bds.E) && Overlapping1D(bdsref.S, bdsref.N, bds.S, bds.N) {
		return true
	} else {
		return false
	}

	return false
}

// area of bds (of a square)
func AreaBds(ext m.Extrema) float64 {
	return (ext.N - ext.S) * (ext.E - ext.W)
}

// feature clip tiled
func PolygonClipTile(feat [][][]float64, tileid m.TileID) map[m.TileID][][][][]float64 {
	// getting bbs as well as poly clip
	poly, bd := MakeBds(feat)

	pt := poly[0][0]

	temptileid := m.Tile(pt.X, pt.Y, int(tileid.Z+1))
	bdtemp := m.Bounds(temptileid)

	// checking to see if the polygon lies entirely within a smaller childd
	if (bd.N <= bdtemp.N) && (bd.S >= bdtemp.S) && (bd.E <= bdtemp.E) && (bd.W >= bdtemp.W) {
		totalmap := map[m.TileID][][][][]float64{}
		totalmap[temptileid] = [][][][]float64{feat}
		return totalmap
	}

	tiles := m.Children(tileid)

	bdtileid := m.Bounds(tileid)
	totalmap := map[m.TileID][][][][]float64{}

	if (math.Abs(AreaBds(bdtileid)-AreaBds(bd)) < math.Pow(.000001, 2.0)) && len(poly) == 1 && len(poly[0]) == 4 {
		//fmt.Print("here\n")

		for _, k := range tiles {
			//poly := Make_Tile_Poly(k)
			bds := m.Bounds(k)

			totalmap[k] = [][][][]float64{{{{bds.E, bds.N}, {bds.W, bds.N}, {bds.W, bds.S}, {bds.E, bds.S}}}}
		}
		return totalmap
	}

	// finally accounting for the worst case scenario
	for _, k := range tiles {
		if Intersect(bd, m.Bounds(k)) {
			totalmap[k] = TileClip(poly, k)
		}
	}
	return totalmap
}

// given a feature return tilemap of the clipped feature
// works for simple geometries
func ClipNaive(feature *geojson.Feature, size int) map[m.TileID][]*geojson.Feature {

	if feature.Geometry == nil {
		return map[m.TileID][]*geojson.Feature{}
	} else if feature.Geometry.Type == "MultiPoint" {
		totaltilemap := map[m.TileID][][]float64{}
		for _, pt := range feature.Geometry.MultiPoint {
			tileid := m.Tile(feature.Geometry.Point[0], feature.Geometry.Point[1], size)
			totaltilemap[tileid] = append(totaltilemap[tileid], pt)
		}
		tilemap := map[m.TileID][]*geojson.Feature{}
		for k, v := range totaltilemap {
			tilemap[k] = []*geojson.Feature{
				&geojson.Feature{Properties: feature.Properties, Geometry: geojson.NewMultiPointGeometry(v...)},
			}
		}
		return tilemap
	} else if feature.Geometry.Type == "MultiLineString" {
		totaltilemap := map[m.TileID][][][]float64{}
		for _, line := range feature.Geometry.MultiLineString {
			temptilemap := ClipLine(line, size)
			for tileid, lines := range temptilemap {
				totaltilemap[tileid] = append(totaltilemap[tileid], lines...)
			}
		}
		totalmap := map[m.TileID][]*geojson.Feature{}
		for k, v := range totaltilemap {
			totalmap[k] = []*geojson.Feature{
				&geojson.Feature{Properties: feature.Properties, Geometry: geojson.NewMultiLineStringGeometry(v...)},
			}
		}
		return totalmap
	} else if feature.Geometry.Type == "MultiPolygon" {
		totaltilemap := map[m.TileID][][][][]float64{}
		for _, polygon := range feature.Geometry.MultiPolygon {
			temptilemap := PolygonClipNaive(polygon, size)
			for tileid, polys := range temptilemap {
				totaltilemap[tileid] = append(totaltilemap[tileid], polys...)
			}
		}
		totalmap := map[m.TileID][]*geojson.Feature{}
		for k, v := range totaltilemap {
			totalmap[k] = []*geojson.Feature{
				&geojson.Feature{Properties: feature.Properties, Geometry: geojson.NewMultiPolygonGeometry(v...)},
			}
		}
		return totalmap
	} else if feature.Geometry.Type == "Point" {
		tileid := m.Tile(feature.Geometry.Point[0], feature.Geometry.Point[1], size)
		return map[m.TileID][]*geojson.Feature{tileid: []*geojson.Feature{feature}}
	} else if feature.Geometry.Type == "LineString" {
		tilemap := ClipLine(feature.Geometry.LineString, size)
		newtilemap := map[m.TileID][]*geojson.Feature{}
		for k, lines := range tilemap {
			newtilemap[k] = make([]*geojson.Feature, len(lines))
			for pos, line := range lines {
				newtilemap[k][pos] = &geojson.Feature{
					Geometry:   geojson.NewLineStringGeometry(line),
					Properties: feature.Properties,
				}
			}
		}
		return newtilemap
	} else if feature.Geometry.Type == "Polygon" {
		tilemap := PolygonClipNaive(feature.Geometry.Polygon, size)
		newtilemap := map[m.TileID][]*geojson.Feature{}
		for k, polygons := range tilemap {
			newtilemap[k] = make([]*geojson.Feature, len(polygons))
			for pos, polygon := range polygons {
				newtilemap[k][pos] = &geojson.Feature{
					Geometry:   geojson.NewPolygonGeometry(polygon),
					Properties: feature.Properties,
				}
			}

		}
		return newtilemap

	}
	return map[m.TileID][]*geojson.Feature{}
}

/*
// clips multi-geometries
func ClipMultiNaive(feature *geojson.Feature, size int) map[m.TileID][]*geojson.Feature {
	var properties map[string]interface{}
	var tempfeature *geojson.Feature
	totalmap := map[m.TileID][]*geojson.Feature{}
	if feature.Geometry.Type == "MultiPoint" {
		for _, pt := range feature.Geometry.MultiPoint {
			temptilemap := ClipNaive(&geojson.Feature{
				Geometry:   geojson.NewPointGeometry(pt),
				Properties: properties,
			}, size)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if boolval {
					tempfeature = temp[0]
				} else {
					tempfeature = &geojson.Feature{Geometry: geojson.NewMultiPointGeometry(v[0].Geometry.Point), Properties: feature.Properties}
				}
				for _, newpt := range v[1:] {
					tempfeature.Geometry.MultiPoint = append(tempfeature.Geometry.MultiPoint, newpt.Geometry.Point)
				}
				totalmap[k] = []*geojson.Feature{tempfeature}
			}
		}
	} else if feature.Geometry.Type == "MultiLineString" {
		for _, line := range feature.Geometry.MultiLineString {
			temptilemap := ClipNaive(&geojson.Feature{
				Geometry:   geojson.NewLineStringGeometry(line),
				Properties: properties,
			}, size)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if boolval {
					tempfeature = temp[0]
				} else {
					tempfeature = &geojson.Feature{Geometry: geojson.NewMultiLineStringGeometry(v[0].Geometry.LineString), Properties: feature.Properties}

				}
				for _, newline := range v[1:] {
					tempfeature.Geometry.MultiLineString = append(tempfeature.Geometry.MultiLineString, newline.Geometry.LineString)
				}
				totalmap[k] = []*geojson.Feature{tempfeature}
			}
		}
	} else if feature.Geometry.Type == "MultiPolygon" {
		for _, polygon := range feature.Geometry.MultiPolygon {
			temptilemap := ClipNaive(&geojson.Feature{
				Geometry:   geojson.NewPolygonGeometry(polygon),
				Properties: properties,
			}, size)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if boolval {
					tempfeature = temp[0]
				} else {
					tempfeature = &geojson.Feature{Geometry: geojson.NewMultiPolygonGeometry(v[0].Geometry.Polygon), Properties: feature.Properties}
				}
				for _, newpolygon := range v[1:] {
					tempfeature.Geometry.MultiPolygon = append(tempfeature.Geometry.MultiPolygon, newpolygon.Geometry.Polygon)
				}
				totalmap[k] = []*geojson.Feature{tempfeature}
			}
		}
	}
	return totalmap
}
*/
/*
// given a feature return tilemap of the clipped feature
// works for simple geometries
// this clipping function clips a tile context
// which is really only useful in the polygon clipping algorithm.
func ClipTile(feature *geojson.Feature, tileid m.TileID) map[m.TileID][]*geojson.Feature {
	// clipping for a single point
	if strings.Contains(string(feature.Geometry.Type), "Multi") {
		//return ClipMultiTile(feature, tileid)
	} else if feature.Geometry.Type == "Point" {
		tileid := m.Tile(feature.Geometry.Point[0], feature.Geometry.Point[1], int(tileid.Z)+1)
		return map[m.TileID][]*geojson.Feature{tileid: []*geojson.Feature{feature}}
	} else if feature.Geometry.Type == "LineString" {
		return ClipLine(feature, int(tileid.Z)+1)
	} else if feature.Geometry.Type == "Polygon" {
		tilemap := PolygonClipTile(feature.Geometry.Polygon, tileid)
		newtilemap := map[m.TileID][]*geojson.Feature{}
		for k, polygons := range tilemap {
			newtilemap[k] = make([]*geojson.Feature, len(polygons))
			for pos, polygon := range polygons {
				newtilemap[k][pos] = &geojson.Feature{
					Geometry:   geojson.NewPolygonGeometry(polygon),
					Properties: feature.Properties,
				}
			}

		}
		return newtilemap
	}
	return map[m.TileID][]*geojson.Feature{}
}
*/
/*
// clips multi-geometries
func ClipMultiTile(feature *geojson.Feature, tile m.TileID) map[m.TileID][]*geojson.Feature {
	var properties map[string]interface{}
	var tempfeature *geojson.Feature
	totalmap := map[m.TileID][]*geojson.Feature{}
	if feature.Geometry.Type == "MultiPoint" {
		for _, pt := range feature.Geometry.MultiPoint {
			temptilemap := ClipTile(&geojson.Feature{
				Geometry:   geojson.NewPointGeometry(pt),
				Properties: properties,
			}, tile)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if len(v) > 0 {
					if boolval {
						tempfeature = temp[0]
					} else {
						tempfeature = &geojson.Feature{Geometry: geojson.NewMultiPointGeometry(v[0].Geometry.Point), Properties: feature.Properties}
					}
					for _, newpt := range v[1:] {
						tempfeature.Geometry.MultiPoint = append(tempfeature.Geometry.MultiPoint, newpt.Geometry.Point)
					}
					totalmap[k] = []*geojson.Feature{tempfeature}
				}
			}
		}
	} else if feature.Geometry.Type == "MultiLineString" {
		for _, line := range feature.Geometry.MultiLineString {
			temptilemap := ClipTile(&geojson.Feature{
				Geometry:   geojson.NewLineStringGeometry(line),
				Properties: properties,
			}, tile)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if len(v) > 0 {
					if boolval {
						tempfeature = temp[0]
					} else {
						tempfeature = &geojson.Feature{Geometry: geojson.NewMultiLineStringGeometry(v[0].Geometry.LineString), Properties: feature.Properties}

					}
					for _, newline := range v[1:] {
						tempfeature.Geometry.MultiLineString = append(tempfeature.Geometry.MultiLineString, newline.Geometry.LineString)
					}
					totalmap[k] = []*geojson.Feature{tempfeature}
				}
			}
		}
	} else if feature.Geometry.Type == "MultiPolygon" {
		for _, polygon := range feature.Geometry.MultiPolygon {
			temptilemap := ClipTile(&geojson.Feature{
				Geometry:   geojson.NewPolygonGeometry(polygon),
				Properties: properties,
			}, tile)

			for k, v := range temptilemap {
				temp, boolval := totalmap[k]
				if len(v) > 0 {
					if boolval {
						tempfeature = temp[0]
					} else {
						tempfeature = &geojson.Feature{Geometry: geojson.NewMultiPolygonGeometry(v[0].Geometry.Polygon), Properties: feature.Properties}
					}
					for _, newpolygon := range v[1:] {
						tempfeature.Geometry.MultiPolygon = append(tempfeature.Geometry.MultiPolygon, newpolygon.Geometry.Polygon)
					}
					totalmap[k] = []*geojson.Feature{tempfeature}
				}
			}
		}
	}
	return totalmap
}
*/

func WriteToBytes(m map[m.TileID][]*geojson.Feature) []byte {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)

	err := encoder.Encode(m)
	if err != nil {
		panic(err)
	}

	// your encoded stuff
	return buf.Bytes()
}

func ReadFromBytes(bytevals []byte) map[m.TileID][]*geojson.Feature {
	buf := bytes.NewBuffer(bytevals)
	var decodedMap map[m.TileID][]*geojson.Feature
	decoder := gob.NewDecoder(buf)

	err := decoder.Decode(&decodedMap)
	if err != nil {
		panic(err)
	}

	return decodedMap
}

func WriteToBytesTest(m map[int]map[m.TileID][]*geojson.Feature) []byte {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)

	err := encoder.Encode(m)
	if err != nil {
		panic(err)
	}

	// your encoded stuff
	return buf.Bytes()
}

func ReadFromBytesTest(bytevals []byte) map[int]map[m.TileID][]*geojson.Feature {
	buf := bytes.NewBuffer(bytevals)
	var decodedMap map[int]map[m.TileID][]*geojson.Feature
	decoder := gob.NewDecoder(buf)

	err := decoder.Decode(&decodedMap)
	if err != nil {
		panic(err)
	}

	return decodedMap
}
