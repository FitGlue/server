package parkrun

// ParkrunLocation represents a known Parkrun event location.
type ParkrunLocation struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// KnownLocations is a map of Parkrun locations where key is a simplified slug/ID.
// In a future iteration, this could be loaded from a JSON file.
// Coordinates are approximate Start Line locations.
var KnownLocations = []ParkrunLocation{
	{
		Name:      "Bushy Park Parkrun",
		Latitude:  51.4106,
		Longitude: -0.3421,
	},
	{
		Name:      "Newark Parkrun",
		Latitude:  53.0697,
		Longitude: -0.8195,
	},
	{
		Name:      "Cardiff Parkrun",
		Latitude:  51.4948,
		Longitude: -3.1933,
	},
	{
		Name:      "Woodhouse Moor Parkrun",
		Latitude:  53.8123,
		Longitude: -1.5658,
	},
	{
		Name:      "Albert Parkrun, Melbourne",
		Latitude:  -37.8427,
		Longitude: 144.9654,
	},
	{
		Name:      "Delta Parkrun, Johannesburg",
		Latitude:  -26.1363,
		Longitude: 28.0163,
	},
	// Add more as needed...
}
