package polyclip

import (
	"github.com/paulmach/go.geojson"
	m "github.com/murphy214/mercantile"
)


func Make(poly [][][]float64) Polygon {
	polygon := make(Polygon,len(poly))
	for i := range poly {
		cont := make(Contour,len(poly[i]))
		for ii,pt := range poly[i] {
			cont[ii] = Point{pt[0],pt[1]}
		}
		polygon[i] = cont
	}
	return Polygon(polygon)
}


func Make_Bds(poly [][][]float64) (Polygon,m.Extrema) {
	north := -1000.
	south := 1000.
	east := -1000.
	west := 1000.
	lat := 0.
	long := 0.
	polygon := make(Polygon,len(poly))
	for i := range poly {
		cont := make(Contour,len(poly[i]))
		for ii,pt := range poly[i] {
			lat = pt[1]
			long = pt[0]

			if lat > north {
				north = lat
			}
			if lat < south {
				south = lat
			}
			if long > east {
				east = long
			}
			if long < west {
				west = long
			}
			cont[ii] = Point{pt[0],pt[1]}
		}
		polygon[i] = cont
	}

	return Polygon(polygon),m.Extrema{S: south, W: west, N: north, E: east}
}


// Overlaps returns whether r1 and r2 have a non-empty intersection.
func Within(big Rectangle, small Rectangle) bool {
	return (big.Min.X <= small.Min.X) && (big.Max.X >= small.Max.X) &&
		(big.Min.Y <= small.Min.Y) && (big.Max.Y >= small.Max.Y)
}

// a check to see if each point of a contour is within the bigger
func WithinAll(big Contour, small Contour) bool {
	totalbool := true
	for _, pt := range small {
		boolval := big.Contains(pt)
		if boolval == false {
			totalbool = false
		}
	}
	return totalbool
}

// creating a list with all of the intersecting contours
// this function returns a list of all the constituent contours as well as
// a list of their keys
func Sweep_Contmap(bb Rectangle, intcont Contour, contmap map[int]Contour) []int {
	newlist := []int{}
	for k, v := range contmap {
		// getting the bounding box
		bbtest := v.BoundingBox()

		// getting within bool
		withinbool := Within(bb, bbtest)

		// logic for if within bool is true
		if withinbool == true {
			withinbool = WithinAll(intcont, v)
		}

		// logic for when we know the contour is within the polygon
		if withinbool == true {
			newlist = append(newlist, k)
		}
	}
	return newlist
}

// getting the outer keys of contours that will be turned into polygons
func make_polygon_list(totalkeys []int, contmap map[int]Contour, relationmap map[int][]int,properties map[string]interface{}) []*geojson.Feature {
	keymap := map[int]string{}
	for _, i := range totalkeys {
		keymap[i] = ""
	}

	// making polygon map
	polygonlist := []*geojson.Feature{}
	for k, v := range contmap {
		_, ok := keymap[k]
		if ok == false {
			newpolygon := Polygon{v}
			otherconts := relationmap[k]
			for _, cont := range otherconts {
				newpolygon.Add(contmap[cont])
			}

			// finally adding to list
			polygonlist = append(polygonlist, 
				&geojson.Feature{
					Geometry:
					geojson.NewPolygonGeometry(
						Convert_Float(newpolygon),
						),
					Properties:properties,
				})
		}
	}
	return polygonlist

}

// creates a within map or a mapping of each edge
func Create_Withinmap(contmap map[int]Contour,properties map[string]interface{}) []*geojson.Feature {
	totalkeys := []int{}
	relationmap := map[int][]int{}
	for k, v := range contmap {
		bb := v.BoundingBox()
		keys := Sweep_Contmap(bb, v, contmap)
		relationmap[k] = keys
		totalkeys = append(totalkeys, keys...)
	}

	return make_polygon_list(totalkeys, contmap, relationmap,properties)
}

// lints each polygon
// takes abstract polygon rings that may contain polygon rings
// and returns geojson arranged polygon sets
func Lint_Polygons(polygon Polygon,properties map[string]interface{}) []*geojson.Feature {
	if len(polygon) == 1 {
		return []*geojson.Feature{&geojson.Feature{
					Geometry:
					geojson.NewPolygonGeometry(
						Convert_Float(polygon),
						),
					Properties:properties,
				}}

	}

	contmap := map[int]Contour{}
	for i, cont := range polygon {
		contmap[i] = cont
	}
	return Create_Withinmap(contmap,properties)
}

// from a Polygon representation (clipping representation)
// to a [][][]float64 representation
func Convert_Float(poly Polygon) [][][]float64 {
	total := make([][][]float64,len(poly))
	for i := range poly {
		size_cont := len(poly[i])
		total[i] = make([][]float64,len(poly[i]))
		for ii := 0; ii < size_cont; ii++ {
			total[i][ii] = []float64{poly[i][ii].X, poly[i][ii].Y}
		}
	}
	return total
}

