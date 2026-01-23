package muscle_heatmap_image

import "embed"

//go:embed muscle_diagram/*.svg
var templatesFS embed.FS

// MusclePathIDs maps muscle group names to SVG path IDs
var MusclePathIDs = map[string]string{
	"chest":      "pectoralis_major",
	"shoulders":  "deltoid",
	"biceps":     "biceps",
	"triceps":    "triceps",
	"forearms":   "brachioradialis",
	"lats":       "latissimus_dorsi",
	"traps":      "trapezius",
	"upper_back": "infraspinatus_teres_major",
	"lower_back": "latissimus_dorsi", // Approximate as discussed
	"abs":        "abdominals",
	"quads":      "quadriceps",
	"hamstrings": "hamstrings",
	"glutes":     "gluteus_maximus", // and gluteus_medius
	"calves":     "gastrocnemius",   // and soleus etc.
}
