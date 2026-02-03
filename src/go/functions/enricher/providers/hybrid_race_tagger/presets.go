package hybrid_race_tagger

// StationType defines whether a station is cardio (lap-based) or strength (set-based)
type StationType int

const (
	StationTypeRun      StationType = iota // Keep as Lap
	StationTypeCardio                      // Keep as Lap with exercise_name
	StationTypeStrength                    // Convert to StrengthSet
)

// Station defines a single station in a hybrid race
type Station struct {
	Name           string      // Exercise name for Hevy mapping
	Type           StationType // How to represent in StandardizedActivity
	DistanceMeters float64     // Expected distance (for cardio/distance-based strength)
	Reps           int32       // Expected reps (for rep-based strength like Wall Balls)
	WeightKg       float64     // Weight in kg (varies by gender/category)
	Icon           string      // Emoji icon for display
}

// RacePreset defines a complete hybrid race format
type RacePreset struct {
	ID          string    // Unique identifier
	Name        string    // Display name
	Description string    // Brief description
	RaceType    string    // e.g., "hyrox", "athx" - used for PR keys
	Stations    []Station // Ordered list of stations
}

// Hyrox weights by category and division
// Reference: https://hyrox.com/race/ (official weight chart)
// Note: Sled weights are "incl. sled" values

// Women's Open
const (
	HyroxWomenOpenSledPushKg = 102.0 // 50kg + 52kg sled
	HyroxWomenOpenSledPullKg = 78.0  // 25kg + 53kg sled
	HyroxWomenOpenFarmersKg  = 16.0  // Per hand (2x16kg)
	HyroxWomenOpenSandbagKg  = 10.0  // Sandbag
	HyroxWomenOpenWallBallKg = 4.0   // Wall Ball
)

// Women's Pro
const (
	HyroxWomenProSledPushKg = 152.0 // 100kg + 52kg sled
	HyroxWomenProSledPullKg = 103.0 // 50kg + 53kg sled
	HyroxWomenProFarmersKg  = 24.0  // Per hand (2x24kg)
	HyroxWomenProSandbagKg  = 20.0  // Sandbag
	HyroxWomenProWallBallKg = 6.0   // Wall Ball
)

// Men's Open
const (
	HyroxMenOpenSledPushKg = 152.0 // 100kg + 52kg sled
	HyroxMenOpenSledPullKg = 103.0 // 50kg + 53kg sled
	HyroxMenOpenFarmersKg  = 24.0  // Per hand (2x24kg)
	HyroxMenOpenSandbagKg  = 20.0  // Sandbag
	HyroxMenOpenWallBallKg = 6.0   // Wall Ball
)

// Men's Pro
const (
	HyroxMenProSledPushKg = 202.0 // 150kg + 52kg sled
	HyroxMenProSledPullKg = 153.0 // 100kg + 53kg sled
	HyroxMenProFarmersKg  = 32.0  // Per hand (2x32kg)
	HyroxMenProSandbagKg  = 30.0  // Sandbag
	HyroxMenProWallBallKg = 9.0   // Wall Ball
)

// hyroxStations creates the standard Hyrox station order with specified weights
func hyroxStations(sledPush, sledPull, farmers, sandbag, wallBall float64) []Station {
	return []Station{
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "SkiErg", Type: StationTypeCardio, DistanceMeters: 1000, Icon: "⛷️"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Sled Push", Type: StationTypeStrength, DistanceMeters: 50, WeightKg: sledPush, Icon: "🛷"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Sled Pull", Type: StationTypeStrength, DistanceMeters: 50, WeightKg: sledPull, Icon: "🛷"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Burpee Broad Jump", Type: StationTypeStrength, DistanceMeters: 80, Icon: "🏋️"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Rowing (Machine)", Type: StationTypeCardio, DistanceMeters: 1000, Icon: "🚣"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Farmers Carry", Type: StationTypeStrength, DistanceMeters: 200, WeightKg: farmers * 2, Icon: "🧳"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Sandbag Lunges", Type: StationTypeStrength, DistanceMeters: 100, WeightKg: sandbag, Icon: "🎒"},
		{Name: "Running (Outdoor)", Type: StationTypeRun, DistanceMeters: 1000, Icon: "🏃"},
		{Name: "Wall Balls", Type: StationTypeStrength, Reps: 100, WeightKg: wallBall, Icon: "🏐"},
	}
}

// RacePresets contains all available hybrid race presets
var RacePresets = map[string]RacePreset{
	// Women's presets
	"hyrox_women_open": {
		ID:          "hyrox_women_open",
		Name:        "Hyrox - Women's Open",
		Description: "Standard Hyrox race for women (Open division)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxWomenOpenSledPushKg, HyroxWomenOpenSledPullKg, HyroxWomenOpenFarmersKg, HyroxWomenOpenSandbagKg, HyroxWomenOpenWallBallKg),
	},
	"hyrox_women_pro": {
		ID:          "hyrox_women_pro",
		Name:        "Hyrox - Women's Pro",
		Description: "Hyrox race for women (Pro division)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxWomenProSledPushKg, HyroxWomenProSledPullKg, HyroxWomenProFarmersKg, HyroxWomenProSandbagKg, HyroxWomenProWallBallKg),
	},
	// Men's presets
	"hyrox_men_open": {
		ID:          "hyrox_men_open",
		Name:        "Hyrox - Men's Open",
		Description: "Standard Hyrox race for men (Open division)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxMenOpenSledPushKg, HyroxMenOpenSledPullKg, HyroxMenOpenFarmersKg, HyroxMenOpenSandbagKg, HyroxMenOpenWallBallKg),
	},
	"hyrox_men_pro": {
		ID:          "hyrox_men_pro",
		Name:        "Hyrox - Men's Pro",
		Description: "Hyrox race for men (Pro division)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxMenProSledPushKg, HyroxMenProSledPullKg, HyroxMenProFarmersKg, HyroxMenProSandbagKg, HyroxMenProWallBallKg),
	},
	// Doubles presets
	"hyrox_doubles_women": {
		ID:          "hyrox_doubles_women",
		Name:        "Hyrox - Women's Doubles",
		Description: "Hyrox doubles race for women (uses Women's Pro weights)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxWomenProSledPushKg, HyroxWomenProSledPullKg, HyroxWomenProFarmersKg, HyroxWomenProSandbagKg, HyroxWomenProWallBallKg),
	},
	"hyrox_doubles_men": {
		ID:          "hyrox_doubles_men",
		Name:        "Hyrox - Men's Doubles",
		Description: "Hyrox doubles race for men (uses Men's Pro weights)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxMenProSledPushKg, HyroxMenProSledPullKg, HyroxMenProFarmersKg, HyroxMenProSandbagKg, HyroxMenProWallBallKg),
	},
	"hyrox_mixed_doubles": {
		ID:          "hyrox_mixed_doubles",
		Name:        "Hyrox - Mixed Doubles",
		Description: "Hyrox race with mixed gender partner (uses Men's Open weights)",
		RaceType:    "hyrox",
		// Mixed doubles uses Men's Open weights
		Stations: hyroxStations(HyroxMenOpenSledPushKg, HyroxMenOpenSledPullKg, HyroxMenOpenFarmersKg, HyroxMenOpenSandbagKg, HyroxMenOpenWallBallKg),
	},
}

// GetPresetList returns a list of preset IDs and names for the UI
func GetPresetList() []struct {
	ID   string
	Name string
} {
	return []struct {
		ID   string
		Name string
	}{
		// Women's categories
		{ID: "hyrox_women_open", Name: "Hyrox - Women's Open"},
		{ID: "hyrox_women_pro", Name: "Hyrox - Women's Pro"},
		// Men's categories
		{ID: "hyrox_men_open", Name: "Hyrox - Men's Open"},
		{ID: "hyrox_men_pro", Name: "Hyrox - Men's Pro"},
		// Doubles categories
		{ID: "hyrox_doubles_women", Name: "Hyrox - Women's Doubles"},
		{ID: "hyrox_doubles_men", Name: "Hyrox - Men's Doubles"},
		{ID: "hyrox_mixed_doubles", Name: "Hyrox - Mixed Doubles"},
	}
}

// GetPreset retrieves a preset by ID
func GetPreset(id string) (RacePreset, bool) {
	preset, ok := RacePresets[id]
	return preset, ok
}
