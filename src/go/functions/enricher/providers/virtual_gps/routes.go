package virtual_gps

import "math"

// LatLong represents a coordinate point
type LatLong struct {
	Lat  float64
	Long float64
}

// RouteDefinition defines a scenic loop
type RouteDefinition struct {
	Name        string
	TotalDistKm float64
	Points      []LatLong
}

// RoutesLibrary holds available routes
var RoutesLibrary = map[string]RouteDefinition{
	"london": {
		Name: "London Hyde Park (Approx)",
		// A rough rectangle around Hyde Park / Kensington Gardens
		Points: []LatLong{
			{51.5028, -0.1513}, // Hyde Park Corner
			{51.5037, -0.1495},
			{51.5065, -0.1505},
			{51.5118, -0.1656}, // Bayswater Road
			{51.5090, -0.1770},
			{51.5080, -0.1830},
			{51.5040, -0.1848}, // Kensington Palace Gdns
			{51.5020, -0.1865},
			{51.4995, -0.1810}, // Kensington Rd
			{51.5005, -0.1710},
			{51.5015, -0.1605}, // Knightsbridge
			{51.5028, -0.1513}, // Back to start
		},
	},
	"nyc": {
		Name: "NYC Central Park Loop",
		// Full Central Park loop, clockwise from 72nd St entrance
		Points: []LatLong{
			{40.7764, -73.9731}, // 72nd St entrance (East)
			{40.7812, -73.9734}, // Conservatory Water
			{40.7851, -73.9745}, // Reservoir south
			{40.7897, -73.9580}, // East 90s
			{40.7968, -73.9549}, // Harlem Meer
			{40.7985, -73.9563}, // North Woods
			{40.7992, -73.9583}, // Great Hill
			{40.7995, -73.9652}, // North Meadow
			{40.7982, -73.9721}, // Pool
			{40.7950, -73.9760}, // West 97th
			{40.7897, -73.9770}, // West 90s
			{40.7851, -73.9780}, // Reservoir west
			{40.7812, -73.9790}, // West 79th
			{40.7764, -73.9795}, // Strawberry Fields
			{40.7735, -73.9760}, // Sheep Meadow
			{40.7688, -73.9735}, // Tavern on the Green
			{40.7678, -73.9720}, // West 65th
			{40.7688, -73.9685}, // Columbus Circle area
			{40.7735, -73.9650}, // Heckscher Playground
			{40.7764, -73.9731}, // Back to 72nd St
		},
	},
}

// calculateTotalDistance computes the total distance of the route in meters
func (r *RouteDefinition) Meters() float64 {
	dist := 0.0
	for i := 0; i < len(r.Points)-1; i++ {
		dist += haversine(r.Points[i], r.Points[i+1])
	}
	return dist
}

// haversine calculates distance between two points in meters
func haversine(p1, p2 LatLong) float64 {
	const earthRadius = 6371000 // meters

	lat1 := p1.Lat * math.Pi / 180
	lat2 := p2.Lat * math.Pi / 180
	dLat := (p2.Lat - p1.Lat) * math.Pi / 180
	dLon := (p2.Long - p1.Long) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
