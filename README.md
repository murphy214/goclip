
# What is it?

Goclip is a general purpose clipping libary that houses a an implementation of polygon clipping library I forked and modified quite a bit as well as a ton of other general purpose clipping algorithms. It is designed to be used with geojson feature objects specifically. 

The main goal behind this clipping library is for tile clipping specifically and there are to main apis for clipping about tiles. Clip_Naive(*geojson.Feature,zoom int) naively clips a feature at a given tile zoom with no context of whats already been clipped or not clipped. It returns a tilemap being a map[m.TileID][]*geojson.Feature type. The other clipping library soley exists for polygons but is implemented at the top level for simplicity. The Clip_Tile(*geojson.Feature,m.TileID) accepts a given geojson feature and the tileid the feature was previously clipped against. This context can provide quite a few polygon clipping speed up with some assumptions. 

# Caveats

Like most libraries I build it currently only support simple geometries being a single geojson feature no multi-feature types currently. 

# To-Do
* Write tests benchmarks
* Depricate old clipping library 
* start thinking about higher order abstractions on top of this
* maybe implement multi-clipping?
