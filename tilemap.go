package polyclip

import (
	"fmt"
	//"fmt"
	g "github.com/murphy214/geobuf"
	m "github.com/murphy214/mercantile"
	"github.com/paulmach/go.geojson"
	"sync"
)

func MakeTilemap(features []*geojson.Feature, size int) map[m.TileID][]*geojson.Feature {
	totaltilemap := map[m.TileID][]*geojson.Feature{}
	count := 0
	c := make(chan map[m.TileID][]*geojson.Feature)
	featuresize := len(features)

	for pos, feature := range features {
		count += 1

		if count < 1000 && pos != featuresize-1 {

			go func(feature *geojson.Feature, c chan map[m.TileID][]*geojson.Feature) {
				c <- ClipNaive(feature, size)
			}(feature, c)
		} else {
			for i := 1; i < count; i++ {

				temptilemap := <-c
				for k, v := range temptilemap {
					totaltilemap[k] = append(totaltilemap[k], v...)
				}
			}
			count = 0

		}

	}
	return totaltilemap
}

/*
func MakeTilemapGeobufMiddle(buf *g.Reader, size int) map[m.TileID][]*geojson.Feature {
	totaltilemap := map[m.TileID][]*geojson.Feature{}
	//count := 0
	//c := make(chan map[m.TileID][]*geojson.Feature)
	//featuresize := len(features)

	for buf.Next() {
		temptilemap := ClipNaiveGeobufMiddle(buf.Bytes(), size)
		for k, v := range temptilemap {
			totaltilemap[k] = append(totaltilemap[k], v...)
		}
	}
	return totaltilemap
}
*/

func MakeTilemapGeobuf(buf *g.Reader, size int) map[m.TileID]*g.Writer {
	totaltilemap := map[m.TileID]*g.Writer{}
	count := 0
	c := make(chan map[m.TileID]*g.Writer)
	var wg sync.WaitGroup
	for buf.Next() {
		count += 1
		bytevals := buf.Bytes()
		wg.Add(1)
		go func(bytevals []byte, c chan map[m.TileID]*g.Writer) {
			defer wg.Done()
			c <- ClipNaiveGeobuf(bytevals, size)
		}(bytevals, c)

		if count == 10000 || buf.Next() == false {
			for i := 0; i < count; i++ {
				// do something useful here
				// the collecitng done here
				temptilemap := <-c

				for k, v := range temptilemap {
					_, boolval := totaltilemap[k]
					if !boolval {
						totaltilemap[k] = g.WriterBufNew()
					}
					//e.AddGeobuf(v)
					totaltilemap[k].AddGeobuf(v)

					fmt.Println(len(totaltilemap))
					//read := totaltilemap[k].Reader()
					//for read.Next() {
					//	fmt.Println(read.Feature())
					//}
				}
			}
			wg.Wait()
			c = make(chan map[m.TileID]*g.Writer)
			count = 0
		}

	}
	for i := 0; i < count; i++ {
		// do something useful here
		// the collecitng done here
		temptilemap := <-c

		for k, v := range temptilemap {
			_, boolval := totaltilemap[k]
			if !boolval {
				totaltilemap[k] = g.WriterBufNew()
			} else {
				//e.AddGeobuf(v)
				totaltilemap[k].AddGeobuf(v)
			}
			fmt.Println(len(totaltilemap))
			//read := totaltilemap[k].Reader()
			//for read.Next() {
			//	fmt.Println(read.Feature())
			//}
		}
	}
	wg.Wait()
	c = make(chan map[m.TileID]*g.Writer)
	count = 0
	return totaltilemap
}
