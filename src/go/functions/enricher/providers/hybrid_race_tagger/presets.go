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

// Hyrox standard weights by category
const (
	// Male Open
	HyroxMaleSledPushKg = 152.0
	HyroxMaleSledPullKg = 103.0
	HyroxMaleFarmersKg  = 24.0 // Per hand (2x24kg)
	HyroxMaleSandbagKg  = 20.0
	HyroxMaleWallBallKg = 9.0

	// Female Open
	HyroxFemaleSledPushKg = 102.0
	HyroxFemaleSledPullKg = 78.0
	HyroxFemaleFarmersKg  = 16.0 // Per hand (2x16kg)
	HyroxFemaleSandbagKg  = 10.0
	HyroxFemaleWallBallKg = 6.0
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
	"hyrox_male_single": {
		ID:          "hyrox_male_single",
		Name:        "Hyrox - Male Single",
		Description: "Standard Hyrox race for male athletes",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxMaleSledPushKg, HyroxMaleSledPullKg, HyroxMaleFarmersKg, HyroxMaleSandbagKg, HyroxMaleWallBallKg),
	},
	"hyrox_female_single": {
		ID:          "hyrox_female_single",
		Name:        "Hyrox - Female Single",
		Description: "Standard Hyrox race for female athletes",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxFemaleSledPushKg, HyroxFemaleSledPullKg, HyroxFemaleFarmersKg, HyroxFemaleSandbagKg, HyroxFemaleWallBallKg),
	},
	"hyrox_doubles": {
		ID:          "hyrox_doubles",
		Name:        "Hyrox - Doubles",
		Description: "Hyrox race with partner (same gender)",
		RaceType:    "hyrox",
		Stations:    hyroxStations(HyroxMaleSledPushKg, HyroxMaleSledPullKg, HyroxMaleFarmersKg, HyroxMaleSandbagKg, HyroxMaleWallBallKg),
	},
	"hyrox_mixed_doubles": {
		ID:          "hyrox_mixed_doubles",
		Name:        "Hyrox - Mixed Doubles",
		Description: "Hyrox race with mixed gender partner",
		RaceType:    "hyrox",
		Stations:    hyroxStations(127, 90.5, 20, 15, 7.5),
	},
}

// GetPresetList returns a list of preset IDs and names for the UI
func GetPresetList() []struct {
	ID   string
	Name string
} {
	list := []struct {
		ID   string
		Name string
	}{
		{ID: "hyrox_male_single", Name: "Hyrox - Male Single"},
		{ID: "hyrox_female_single", Name: "Hyrox - Female Single"},
		{ID: "hyrox_doubles", Name: "Hyrox - Doubles"},
		{ID: "hyrox_mixed_doubles", Name: "Hyrox - Mixed Doubles"},
	}
	return list
}

// GetPreset retrieves a preset by ID
func GetPreset(id string) (RacePreset, bool) {
	preset, ok := RacePresets[id]
	return preset, ok
}
