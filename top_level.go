package polyclip

import (
	"github.com/paulmach/go.geojson"
	m "github.com/murphy214/mercantile"
	"math"
)


func Clip(subject *geojson.Feature,about *geojson.Feature,operation Op) []*geojson.Feature {
	return Lint_Polygons(
		Make(subject.Geometry.Polygon).Construct(
			operation,
			Make(about.Geometry.Polygon),
		),
		map[string]interface{}{},
	)
}


// tile clipping
func Tile_Clip(about Polygon,properties map[string]interface{},tile m.TileID) []*geojson.Feature {
	bds := m.Bounds(tile)
	return Lint_Polygons(Polygon{
			{Point{bds.E, bds.N},
			 Point{bds.W, bds.N}, 
			 Point{bds.W, bds.S}, 
			 Point{bds.E, bds.S},
			},
		}.Construct(
			INTERSECTION,
			about,
		),
		properties,
	)
}

func Create_BBs(minx,maxx,miny,maxy,zoom int) []m.TileID {
	tileids := make([]m.TileID,(maxx - minx + 1) * (maxy - miny + 1))
	pos := 0
	for currentx := minx; currentx <= maxx; currentx++ {
		for currenty := miny; currenty <= maxy; currenty++ {
			tileids[pos] = m.TileID{int64(currentx),int64(currenty),uint64(zoom)}
			pos++
		}
	}
	return tileids
}

type Output struct {
	TileID m.TileID
	Features []*geojson.Feature
}


func Feature_Clip_Naive(feat *geojson.Feature,zoom int) map[m.TileID][]*geojson.Feature {
	// getting bbs as well as poly clip
	poly,bds := Make_Bds(feat.Geometry.Polygon)
	// getting all four corners
	c1 := Point{bds.E, bds.N}
	c3 := Point{bds.W, bds.S}
	c1t := m.Tile(c1.X, c1.Y, zoom)	
	c3t := m.Tile(c3.X, c3.Y, zoom)	

	minx,maxx := int(math.Min(float64(c1t.X),float64(c3t.X))),int(math.Max(float64(c1t.X),float64(c3t.X)))
	miny,maxy := int(math.Min(float64(c1t.Y),float64(c3t.Y))),int(math.Max(float64(c1t.Y),float64(c3t.Y)))

	bbs := Create_BBs(minx,maxx,miny,maxy,zoom)
	properties := feat.Properties
	c := make(chan Output)
	for _,bb := range bbs { 
		go func(poly Polygon,bb m.TileID,c chan Output) {
			c <- Output{bb,Tile_Clip(poly,properties,bb)}
		}(poly,bb,c)
	}

	totalmap := map[m.TileID][]*geojson.Feature{}
	for range bbs {
		out := <-c
		if len(out.Features) > 0 {
			totalmap[out.TileID] = out.Features
		}
	}
	return totalmap
}

// structure for finding overlapping values
func Overlapping_1D(box1min float64,box1max float64,box2min float64,box2max float64) bool {
	if box1max >= box2min && box2max >= box1min {
		return true
	} else {
		return false
	}
	return false
}


// returns a boolval for whether or not the bb intersects
func Intersect(bdsref m.Extrema,bds m.Extrema) bool {
	if Overlapping_1D(bdsref.W,bdsref.E,bds.W,bds.E) && Overlapping_1D(bdsref.S,bdsref.N,bds.S,bds.N) {
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
func Feature_Clip_Tile(feat *geojson.Feature,tileid m.TileID) map[m.TileID][]*geojson.Feature {
	// getting bbs as well as poly clip
	poly,bd := Make_Bds(feat.Geometry.Polygon)

	pt := poly[0][0]

	temptileid := m.Tile(pt.X, pt.Y, int(tileid.Z+1))
	bdtemp := m.Bounds(temptileid)

	// checking to see if the polygon lies entirely within a smaller childd
	if (bd.N <= bdtemp.N) && (bd.S >= bdtemp.S) && (bd.E <= bdtemp.E) && (bd.W >= bdtemp.W) {
		totalmap := map[m.TileID][]*geojson.Feature{}
		totalmap[temptileid] = []*geojson.Feature{feat}
		return totalmap
	}

	tiles := m.Children(tileid)

	bdtileid := m.Bounds(tileid)
	totalmap := map[m.TileID][]*geojson.Feature{}

	if (math.Abs(AreaBds(bdtileid)-AreaBds(bd)) < math.Pow(.000001,2.0)) && len(poly) == 1 && len(poly[0]) == 4 {
		//fmt.Print("here\n")

		for _, k := range tiles {
			//poly := Make_Tile_Poly(k)
			bds := m.Bounds(k)
			
			totalmap[k] = []*geojson.Feature{
				&geojson.Feature{
					Geometry:geojson.NewPolygonGeometry([][][]float64{{{bds.E, bds.N}, {bds.W, bds.N}, {bds.W, bds.S}, {bds.E, bds.S}}}),
					Properties:feat.Properties,
				},
			}
		}
		return totalmap
	}

	// finally accounting for the worst case scenario
	for _,k := range tiles { 
		if Intersect(bd,m.Bounds(k)) {
			totalmap[k] = Tile_Clip(poly,feat.Properties,k)
		}
	}
	return totalmap
}

// given a feature return tilemap of the clipped feature
// works for simple geometries
func Clip_Naive(feature *geojson.Feature,size int) map[m.TileID][]*geojson.Feature {
	// clipping for a single point
	if feature.Geometry.Type == "Point" {
		tileid := m.Tile(feature.Geometry.Point[0],feature.Geometry.Point[1],size)
		return map[m.TileID][]*geojson.Feature{tileid:[]*geojson.Feature{feature}}
	} else if feature.Geometry.Type == "LineString" {
		return Clip_Line(feature,size)
	} else if feature.Geometry.Type == "Polygon" {
		return Feature_Clip_Naive(feature,size)
	}
	return map[m.TileID][]*geojson.Feature{}
}
 
// given a feature return tilemap of the clipped feature
// works for simple geometries
// this clipping function clips a tile context 
// which is really only useful in the polygon clipping algorithm.
func Clip_Tile(feature *geojson.Feature,tileid m.TileID) map[m.TileID][]*geojson.Feature {
	// clipping for a single point
	if feature.Geometry.Type == "Point" {
		tileid := m.Tile(feature.Geometry.Point[0],feature.Geometry.Point[1],int(tileid.Z)+1)
		return map[m.TileID][]*geojson.Feature{tileid:[]*geojson.Feature{feature}}
	} else if feature.Geometry.Type == "LineString" {
		return Clip_Line(feature,int(tileid.Z)+1)
	} else if feature.Geometry.Type == "Polygon" {
		return Feature_Clip_Tile(feature,tileid)
	}
	return map[m.TileID][]*geojson.Feature{}
}