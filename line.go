package polyclip

import (
	m "github.com/murphy214/mercantile"
	"math"
	"sort"
	"github.com/paulmach/go.geojson"
)

func Get_Corners(bds m.Extrema) ([]float64,[]float64,[]float64,[]float64) {
	wn := []float64{bds.W,bds.N}
	ws := []float64{bds.W,bds.S}
	en := []float64{bds.E,bds.N}
	es := []float64{bds.E,bds.S}
	return wn,ws,en,es
}

func Get_Min_Max(bds m.Extrema,zoom int) (int,int,int,int) {
	// getting corners
	wn,_,_,es := Get_Corners(bds)

	wnt := m.Tile(wn[0],wn[1],zoom)
	est := m.Tile(es[0],es[1],zoom)
	minx,maxx,miny,maxy := int(wnt.X),int(est.X),int(wnt.Y),int(est.Y)
	return minx,maxx,miny,maxy
}

// assembles a set of ranges
func Between(minval,maxval int) []int {
	current := minval 
	newlist := []int{current}
	for current < maxval {
		current += 1
		newlist = append(newlist,current)
	}
	return newlist
}

// getting tiles that cover the bounding box
func Get_BB_Tiles(bds m.Extrema,zoom int) []m.TileID {
	// the getting min and max
	minx,maxx, miny,maxy := Get_Min_Max(bds,zoom)
	
	// getting xs and ys
	xs,ys := Between(minx,maxx),Between(miny,maxy)

	// gertting all tiles
	newlist := []m.TileID{}
	for _,x := range xs {
		for _,y := range ys {
			newlist = append(newlist,m.TileID{int64(x),int64(y),uint64(zoom)})
		}
	}
	return newlist
}

// regular interpolation
func Interpolate(pt1,pt2 []float64,x float64) []float64 {
	slope := (pt2[1] - pt1[1]) / (pt2[0] - pt1[0])
	if math.Abs(slope) == math.Inf(1) {
		return []float64{pt1[0],(pt1[1]+pt2[1])/2.0}
	}
	return []float64{x,(x - pt1[0]) * slope + pt1[1]}

}


func Sort_Pts(pts [][]float64,lowest_first bool,xbool bool) [][]float64 {
	
	floatmap := map[float64][]float64{}
	floatlist := make([]float64,len(pts))

	for pos,pt := range pts {
		if xbool {
			floatlist[pos] = pt[0]
			floatmap[pt[0]] = pt
		} else {
			floatlist[pos] = pt[1]
			floatmap[pt[1]] = pt
		}

	}
	sort.Float64s(floatlist)

	if lowest_first {

	} else {
	    for i, j := 0, len(floatlist)-1; i < j; i, j = i+1, j-1 {
	        floatlist[i], floatlist[j] = floatlist[j], floatlist[i]
	    }
	}

	// iterating through sorted floatlist
	// iterating through sorted floatlist
	newsegs := [][]float64{}
	for _,k := range floatlist {
		newsegs = append(newsegs,floatmap[k])
	}
	return newsegs
}



// interpolates points
func Interpolate_Pts(pt1,pt2 []float64,zoom int) [][]float64 {
	var bds m.Extrema
	if pt1[0] >= pt2[0] {
		bds.E = pt1[0]
		bds.W = pt2[0]
	} else {
		bds.W = pt1[0]
		bds.E = pt2[0]
	}
	if pt1[1] >= pt2[1] {
		bds.N = pt1[1]
		bds.S = pt2[1]
	} else {
		bds.S = pt1[1]
		bds.N = pt2[1]
	}
	newpts := [][]float64{}
	pts := [][]float64{}

	minx,maxx,miny,maxy := Get_Min_Max(bds,zoom)
	for _,x := range Between(minx,maxx) {
		tmpbds := m.Bounds(m.TileID{int64(x),int64(miny),uint64(zoom)})
		x1,x2 := tmpbds.W+.00000001,tmpbds.E-.00000001
		//x1,x2 := tmpbds.W ,tmpbds.E

		if bds.W <= x1 && bds.E >= x1 {
			pt := Interpolate(pt1,pt2,x1)
			pts = append(pts,pt)
		}
		if bds.W <= x2 && bds.E >= x2 {
			pt := Interpolate(pt1,pt2,x2)
			pts = append(pts,pt)
		}
	}
	pt1b := []float64{pt1[1],pt1[0]}
	pt2b := []float64{pt2[1],pt2[0]}

	for _,y := range Between(miny,maxy) {
		tmpbds := m.Bounds(m.TileID{int64(minx),int64(y),uint64(zoom)})
		y1,y2 := tmpbds.S+.00000001,tmpbds.N-.00000001
		//y1,y2 := tmpbds.S ,tmpbds.N

		if bds.S <= y1 && bds.N >= y1 {
			pt := Interpolate(pt1b,pt2b,y1)
			pt = []float64{pt[1],pt[0]}
			pts = append(pts,pt)
		}
		if bds.S <= y2 && bds.N >= y2 {
			pt := Interpolate(pt1b,pt2b,y2)
			pt = []float64{pt[1],pt[0]}
			pts = append(pts,pt)
		}
	}
	//pts = append(pts,pt1)
	//pts = append(pts,pt2)

	if pt1[0] > pt2[0] {
		newpts = Sort_Pts(pts,false,true)
	} else if pt1[0] != pt2[0] {
		newpts = Sort_Pts(pts,true,true)		
	} else {
		if pt1[1] > pt2[1] {
			newpts = Sort_Pts(pts,true,false)

		} else {
			newpts = Sort_Pts(pts,false,false)
		}
	}

	return newpts
}

func Get_Avg(pt1,pt2 []float64) []float64 {
	return []float64{(pt1[0] + pt2[0]) / 2.0,(pt1[1] + pt2[1]) / 2.0}
}


func Clip_Line(feat *geojson.Feature,size int) map[m.TileID][]*geojson.Feature {

	tilemap := map[m.TileID][]*geojson.Feature{}
	oldpt := feat.Geometry.LineString[0]
	current_line := [][]float64{oldpt}

	var oldtileid,tileid m.TileID
	oldtileid = m.Tile(oldpt[0],oldpt[1],size)
	for _,pt := range feat.Geometry.LineString {
		tileid = m.Tile(pt[0],pt[1],size)
		// handle the newline creation here
		if oldtileid != tileid {
			between_pts := Interpolate_Pts(oldpt,pt,size)
			// if only one inbetween point
			if len(between_pts) == 1 {
				between := between_pts[0]
				current_line = append(current_line,between)
				tilemap[oldtileid] = append(tilemap[oldtileid],
					&geojson.Feature{Geometry:geojson.NewLineStringGeometry(current_line),
						Properties:feat.Properties,
					})
				current_line = [][]float64{between,pt}
			} else {
				var oldbetweenpt []float64
				// if there are more than one inbetween pt
				for pos,betweenpt := range between_pts {
					if pos == 0 {
						current_line = append(current_line,betweenpt)
						tilemap[oldtileid] = append(tilemap[oldtileid],
							&geojson.Feature{Geometry:geojson.NewLineStringGeometry(current_line),
								Properties:feat.Properties,
							})
						current_line = [][]float64{betweenpt}

					} else {
						current_line = append(current_line,betweenpt)
						avgpt := Get_Avg(oldbetweenpt,betweenpt)
						tileidval := m.Tile(avgpt[0],avgpt[1],size)
						tilemap[tileidval] = append(tilemap[tileidval],
							&geojson.Feature{Geometry:geojson.NewLineStringGeometry(current_line),
								Properties:feat.Properties,
							})
						current_line = [][]float64{betweenpt}
					}
					oldbetweenpt = betweenpt
				}
				current_line = append(current_line,pt)
			}

		} else {
			current_line = append(current_line,oldpt)
		}
		oldtileid = tileid
		oldpt = pt
	}
	// adding the last aprt of the lien
	current_line = append(current_line,oldpt)
	tilemap[oldtileid] = append(tilemap[oldtileid],
		&geojson.Feature{Geometry:geojson.NewLineStringGeometry(current_line),
			Properties:feat.Properties,
		})
	return tilemap

}
