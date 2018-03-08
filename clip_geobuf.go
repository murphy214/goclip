package polyclip

import (
	//"fmt"
	g "github.com/murphy214/geobuf"
	"github.com/murphy214/geobuf/geobuf_raw"
	m "github.com/murphy214/mercantile"
	"github.com/murphy214/pbf"
	"math"
	//"github.com/paulmach/go.geojson"
)

func RoundPt(pt []float64) []float64 {
	return []float64{Round(pt[0], .5, 6), Round(pt[1], .5, 6)}
}

func RoundPolygon(polygon [][][]float64) [][][]float64 {
	for i := range polygon {
		cont := polygon[i]
		for j := range cont {
			cont[j] = RoundPt(cont[j])
		}
		lastpt := cont[len(cont)-1]
		if cont[0][0] != lastpt[0] || cont[0][1] != lastpt[1] {
			cont[len(cont)-1] = cont[0]
		}

		polygon[i] = cont
	}
	return polygon
}

// adding a geobuf byte array to a given layer
// this function house's both the ingestion and output to vector tiles
// hopefully to reduce allocations
func ClipNaiveGeobuf(bytevals []byte, zoom int) map[m.TileID]*g.Writer {

	// the pbf representing a feauture
	pbf := pbf.PBF{Pbf: bytevals, Length: len(bytevals)}

	// creating total bytes that holds the bytes for a given layer
	// refreshing cursor

	key, val := pbf.ReadKey()

	if key == 1 && val == 0 {
		pbf.ReadVarint()
		key, val = pbf.ReadKey()
	}
	for key == 2 && val == 2 {
		// starting properties shit here
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size
		pbf.Pos = endpos
		key, val = pbf.ReadKey()
	}
	var geomtype string
	if key == 3 && val == 0 {
		switch int(pbf.Pbf[pbf.Pos]) {
		case 1:
			geomtype = "Point"
		case 2:
			geomtype = "LineString"
		case 3:
			geomtype = "Polygon"
		case 4:
			geomtype = "MultiPoint"
		case 5:
			geomtype = "MultiLineString"
		case 6:
			geomtype = "MultiPolygon"
		}
		pbf.Pos += 1
		key, val = pbf.ReadKey()
	}
	endpos := pbf.Pos
	bytevals = bytevals[:endpos-1]
	if key == 4 && val == 2 {
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size

		switch geomtype {
		case "Point":
			point := pbf.ReadPoint(endpos)

			tilemap := map[m.TileID][][]float64{m.Tile(point[0], point[1], zoom): [][]float64{point}}
			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _, point := range v {
					geomb := geobuf_raw.MakePoint(point)
					newtilemap[k].Write(append(bytevals, geomb...))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "LineString":
			tilemap := ClipLine(pbf.ReadLine(0, endpos), zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _, line := range v {
					geomb, _ := geobuf_raw.MakeLine(line)
					newtilemap[k].Write(append(bytevals, geomb...))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "Polygon":
			poly := pbf.ReadPolygon(endpos)
			tilemap := PolygonClipNaive(poly, zoom)

			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _, polygon := range v {
					geomb, _ := geobuf_raw.MakePolygon(polygon)
					newtilemap[k].Write(append(bytevals, geomb...))

				}

			}

			return newtilemap
		case "MultiPoint":
			points := pbf.ReadLine(0, endpos)
			tilemap := map[m.TileID][][]float64{}
			for _, point := range points {
				tilemap[m.Tile(point[0], point[1], zoom)] = append(tilemap[m.Tile(point[0], point[1], zoom)], point)
			}
			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				geomb, _ := geobuf_raw.MakeLine(v)
				newtilemap[k].Write(append(bytevals, geomb...))

			}
			return newtilemap
		case "MultiLineString":
			lines := pbf.ReadPolygon(endpos)
			tilemap := map[m.TileID][][][]float64{}
			for _, line := range lines {
				templinemap := ClipLine(line, zoom)
				for k, v := range templinemap {
					tilemap[k] = append(tilemap[k], v...)
				}
			}

			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				geomb, _ := geobuf_raw.MakePolygon(v)
				newtilemap[k].Write(append(bytevals, geomb...))

			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiPolygon":
			multipolygon := pbf.ReadMultiPolygon(endpos)
			tilemap := map[m.TileID][][][][]float64{}
			for _, polygon := range multipolygon {
				temppolygonmap := PolygonClipNaive(polygon, zoom)
				for k, v := range temppolygonmap {
					tilemap[k] = append(tilemap[k], v...)
				}
			}

			newtilemap := map[m.TileID]*g.Writer{}
			for k, v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				geomb, _ := geobuf_raw.MakeMultiPolygon(v)
				newtilemap[k].Write(append(bytevals, geomb...))

			}
			return newtilemap
			//layer.Cursor.MakeMultiPolygonFloat(pbf.ReadMultiPolygon(endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		}
	}
	return map[m.TileID]*g.Writer{}
}

func DeltaPt(pt []float64, testpt []float64) float64 {
	deltax := math.Abs(pt[0] - testpt[0])
	deltay := math.Abs(pt[1] - testpt[1])
	return deltax + deltay
}

var PrecisionError = math.Pow(10.0, -7.0)

/*
// adding a geobuf byte array to a given layer
// this function house's both the ingestion and output to vector tiles
// hopefully to reduce allocations
func ClipNaiveGeobufMiddle(bytevals []byte, zoom int) map[m.TileID][]*geojson.Feature {

	// the pbf representing a feauture
	pbf := pbf.PBF{Pbf: bytevals, Length: len(bytevals)}

	// creating total bytes that holds the bytes for a given layer
	// refreshing cursor

	key, val := pbf.ReadKey()

	if key == 1 && val == 0 {
		pbf.ReadVarint()
		key, val = pbf.ReadKey()
	}
	for key == 2 && val == 2 {
		// starting properties shit here
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size
		pbf.Pos = endpos
		key, val = pbf.ReadKey()
	}
	var geomtype string
	if key == 3 && val == 0 {
		switch int(pbf.Pbf[pbf.Pos]) {
		case 1:
			geomtype = "Point"
		case 2:
			geomtype = "LineString"
		case 3:
			geomtype = "Polygon"
		case 4:
			geomtype = "MultiPoint"
		case 5:
			geomtype = "MultiLineString"
		case 6:
			geomtype = "MultiPolygon"
		}
		pbf.Pos += 1
		key, val = pbf.ReadKey()
	}
	//endpos := pbf.Pos
	//bytevals2 := bytevals[:endpos]
	if key == 4 && val == 2 {
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size

		switch geomtype {
		case "Point":
			point := pbf.ReadPoint(endpos)

			tilemap := map[m.TileID][][]float64{m.Tile(point[0], point[1], zoom): [][]float64{point}}
			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {
				for _, point := range v {
					///geomb := geobuf_raw.MakePoint(point)
					newtilemap[k] = append(newtilemap[k], geojson.NewPointFeature(point))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "LineString":
			tilemap := ClipLine(pbf.ReadLine(0, endpos), zoom)
			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {
				for _, line := range v {
					newtilemap[k] = append(newtilemap[k], geojson.NewMultiLineStringFeature(line))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "Polygon":
			tilemap := PolygonClipNaive(pbf.ReadPolygon(endpos), zoom)
			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {

				for _, polygon := range v {
					if len(v) > 0 {
						newtilemap[k] = append(newtilemap[k], geojson.NewPolygonFeature(polygon))
					}
				}
			}
			return newtilemap
			//layer.Cursor.MakePolygonFloat(pbf.ReadPolygon(endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiPoint":
			points := pbf.ReadLine(0, endpos)
			tilemap := map[m.TileID][][]float64{}
			for _, point := range points {
				tilemap[m.Tile(point[0], point[1], zoom)] = append(tilemap[m.Tile(point[0], point[1], zoom)], point)
			}
			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {
				newtilemap[k] = append(newtilemap[k], geojson.NewMultiPointFeature(v...))
			}
			return newtilemap
			//layer.Cursor.MakeMultiPointFloat(pbf.ReadLine(0,endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiLineString":
			lines := pbf.ReadPolygon(endpos)
			tilemap := map[m.TileID][][][]float64{}
			for _, line := range lines {
				templinemap := ClipLine(line, zoom)
				for k, v := range templinemap {
					tilemap[k] = append(tilemap[k], v...)
				}
			}

			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {
				newtilemap[k] = append(newtilemap[k], geojson.NewMultiLineStringFeature(v...))
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiPolygon":
			multipolygon := pbf.ReadMultiPolygon(endpos)
			tilemap := map[m.TileID][][][][]float64{}
			for _, polygon := range multipolygon {
				temppolygonmap := PolygonClipNaive(polygon, zoom)
				for k, v := range temppolygonmap {
					tilemap[k] = append(tilemap[k], v...)
				}
			}

			newtilemap := map[m.TileID][]*geojson.Feature{}
			for k, v := range tilemap {
				newtilemap[k] = append(newtilemap[k], geojson.NewMultiPolygonFeature(v...))

			}
			return newtilemap
			//layer.Cursor.MakeMultiPolygonFloat(pbf.ReadMultiPolygon(endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		}
	}
	return map[m.TileID][]*geojson.Feature{}
}
*/
/*()
// adding a geobuf byte array to a given layer
// this function house's both the ingestion and output to vector tiles
// hopefully to reduce allocations
func ClipTileGeobuf(bytevals []byte,zoom m.TileID) map[m.TileID]*g.Writer {

	// the pbf representing a feauture
	pbf := geobuf_raw.PBF{Pbf:bytevals,Length:len(bytevals)}

	// creating total bytes that holds the bytes for a given layer
	// refreshing cursor

	key,val := pbf.ReadKey()

	if key == 1 && val == 0 {
		pbf.ReadVarint()
		key,val = pbf.ReadKey()
	}
	for key == 2 && val == 2 {
		// starting properties shit here
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size
		pbf.Pos = endpos
		key,val = pbf.ReadKey()
	}
	var geomtype string
	if key == 3 && val == 0 {
		switch int(pbf.Pbf[pbf.Pos]) {
		case 1:
			geomtype = "Point"
		case 2:
			geomtype = "LineString"
		case 3:
			geomtype = "Polygon"
		case 4:
			geomtype = "MultiPoint"
		case 5:
			geomtype = "MultiLineString"
		case 6:
			geomtype = "MultiPolygon"
		}
		pbf.Pos += 1
		key,val = pbf.ReadKey()
	}
	endpos := pbf.Pos
	bytevals = bytevals[:endpos-1]
	if key == 4 && val == 2 {
		size := pbf.ReadVarint()
		endpos := pbf.Pos + size

		switch geomtype {
		case "Point":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewPointGeometry(pbf.ReadPoint(endpos))),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _,feat := range v {
					geomb := geobuf_raw.MakePoint(feat.Geometry.Point)
					newtilemap[k].Write(append(bytevals,geomb...))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "LineString":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewLineStringGeometry(pbf.ReadLine(0,endpos))),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				eh := g.WriterBufNew()
				for _,feat := range v {
					geomb,_ := geobuf_raw.MakeLine(feat.Geometry.LineString)
					eh.Write(append(bytevals,geomb...))
				}
				newtilemap[k] = eh
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "Polygon":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewPolygonGeometry(pbf.ReadPolygon(endpos))),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				eh := g.WriterBufNew()

				for _,feat := range v {
					if len(v) > 0 {
						geomb,_ := geobuf_raw.MakePolygon(feat.Geometry.Polygon)
						eh.Write(append(bytevals,geomb...))
					}
				}
				newtilemap[k] = eh

			}
			return newtilemap
			//layer.Cursor.MakePolygonFloat(pbf.ReadPolygon(endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiPoint":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewMultiPointGeometry(pbf.ReadLine(0,endpos)...)),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _,feat := range v {
					geomb,_ := geobuf_raw.MakeLine(feat.Geometry.MultiPoint)
					newtilemap[k].Write(append(bytevals,geomb...))
				}
			}
			return newtilemap
			//layer.Cursor.MakeMultiPointFloat(pbf.ReadLine(0,endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiLineString":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewMultiLineStringGeometry(pbf.ReadPolygon(endpos)...)),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _,feat := range v {
					geomb,_ := geobuf_raw.MakePolygon(feat.Geometry.MultiLineString)
					newtilemap[k].Write(append(bytevals,geomb...))
				}
			}
			return newtilemap
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		case "MultiPolygon":
			tilemap := ClipTile(geojson.NewFeature(geojson.NewMultiPolygonGeometry(pbf.ReadMultiPolygon(endpos)...)),zoom)
			newtilemap := map[m.TileID]*g.Writer{}
			for k,v := range tilemap {
				newtilemap[k] = g.WriterBufNew()
				for _,feat := range v {
					geomb,_ := geobuf_raw.MakeMultiPolygon(feat.Geometry.MultiPolygon)
					newtilemap[k].Write(append(bytevals,geomb...))
				}
			}
			return newtilemap
			//layer.Cursor.MakeMultiPolygonFloat(pbf.ReadMultiPolygon(endpos))
			//array9 = WritePackedUint32(layer.Cursor.Geometry)
		}
	}
	return map[m.TileID]*g.Writer{}
}
*/
