package polyclip

import (
	pc "./polyclip"
	m "github.com/murphy214/mercantile"
	//"github.com/paulmach/go.geojson"
)

func Make(poly [][][]float64) pc.Polygon {
	polygon := make(pc.Polygon, len(poly))
	for i := range poly {
		cont := make(pc.Contour, len(poly[i]))
		for ii, pt := range poly[i] {
			pt = RoundPt(pt)
			cont[ii] = pc.Point{pt[0], pt[1]}
		}
		lastpt := cont[len(cont)-1]
		if cont[0].X != lastpt.X || cont[0].Y != lastpt.Y {
			cont[len(cont)-1] = cont[0]
		}
		polygon[i] = cont
	}
	return pc.Polygon(polygon)
}

func MakeBds(poly [][][]float64) (pc.Polygon, m.Extrema) {
	north := -1000.
	south := 1000.
	east := -1000.
	west := 1000.
	lat := 0.
	long := 0.
	polygon := make(pc.Polygon, len(poly))
	for i := range poly {
		cont := make(pc.Contour, len(poly[i]))
		for ii, pt := range poly[i] {
			pt = RoundPt(pt)

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
			cont[ii] = pc.Point{pt[0], pt[1]}
		}
		lastpt := cont[len(cont)-1]
		if cont[0].X != lastpt.X || cont[0].Y != lastpt.Y {
			cont[len(cont)-1] = cont[0]
		}
		polygon[i] = cont
	}

	return pc.Polygon(polygon), m.Extrema{S: south, W: west, N: north, E: east}
}

// Overlaps returns whether r1 and r2 have a non-empty intersection.
func Within(big pc.Rectangle, small pc.Rectangle) bool {
	return (big.Min.X <= small.Min.X) && (big.Max.X >= small.Max.X) &&
		(big.Min.Y <= small.Min.Y) && (big.Max.Y >= small.Max.Y)
}

// a check to see if each point of a contour is within the bigger
func WithinAll(big pc.Contour, small pc.Contour) bool {
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
func SweepContmap(bb pc.Rectangle, intcont pc.Contour, contmap map[int]pc.Contour) []int {
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
func make_polygon_list(totalkeys []int, contmap map[int]pc.Contour, relationmap map[int][]int) [][][][]float64 {
	keymap := map[int]string{}
	for _, i := range totalkeys {
		keymap[i] = ""
	}

	// making polygon map
	polygonlist := [][][][]float64{}
	for k, v := range contmap {
		_, ok := keymap[k]
		if ok == false {
			newpolygon := pc.Polygon{v}
			otherconts := relationmap[k]
			for _, cont := range otherconts {
				newpolygon.Add(contmap[cont])
			}

			// finally adding to list
			polygonlist = append(polygonlist, ConvertFloat(newpolygon))
		}
	}
	return polygonlist

}

// creates a within map or a mapping of each edge
func CreateWithinmap(contmap map[int]pc.Contour) [][][][]float64 {
	totalkeys := []int{}
	relationmap := map[int][]int{}
	for k, v := range contmap {
		bb := v.BoundingBox()
		keys := SweepContmap(bb, v, contmap)
		relationmap[k] = keys
		totalkeys = append(totalkeys, keys...)
	}

	return make_polygon_list(totalkeys, contmap, relationmap)
}

// lints each polygon
// takes abstract polygon rings that may contain polygon rings
// and returns geojson arranged polygon sets
func LintPolygons(polygon pc.Polygon) [][][][]float64 {
	if len(polygon) == 1 {
		return [][][][]float64{ConvertFloat(polygon)}
	}

	contmap := map[int]pc.Contour{}
	for i, cont := range polygon {
		contmap[i] = cont
	}
	return CreateWithinmap(contmap)
}

// from a Polygon representation (clipping representation)
// to a [][][]float64 representation
func ConvertFloat(poly pc.Polygon) [][][]float64 {
	total := make([][][]float64, len(poly))
	for i := range poly {
		size_cont := len(poly[i])
		total[i] = make([][]float64, len(poly[i]))
		for ii := 0; ii < size_cont; ii++ {
			total[i][ii] = []float64{poly[i][ii].X, poly[i][ii].Y}
		}
	}
	return total
}
