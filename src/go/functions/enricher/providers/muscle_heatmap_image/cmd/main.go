package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fitglue/server/src/go/functions/enricher/providers/muscle_heatmap_image"
)

func main() {
	provider := muscle_heatmap_image.NewMuscleHeatmapImageProvider()
	outputDir := "examples_output"

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		panic(err)
	}

	scenarios := []struct {
		Filename string
		Gender   string
		Scores   []muscle_heatmap_image.MuscleScore
	}{
		{
			Filename: "man-chest-day.svg",
			Gender:   "man",
			Scores: []muscle_heatmap_image.MuscleScore{
				{SVGIDs: []string{"pectoralis_major"}, Percentage: 1.0, Color: "#EC4899"}, // Pink (max)
				{SVGIDs: []string{"triceps"}, Percentage: 0.7, Color: "#D946EF"},          // Magenta (high)
				{SVGIDs: []string{"deltoid"}, Percentage: 0.5, Color: "#8B5CF6"},          // Purple (med)
			},
		},
		{
			Filename: "woman-leg-day.svg",
			Gender:   "woman",
			Scores: []muscle_heatmap_image.MuscleScore{
				{SVGIDs: []string{"quadriceps", "sartorius_abductors", "tensor_fasciae_latae"}, Percentage: 1.0, Color: "#EC4899"},
				{SVGIDs: []string{"hamstrings"}, Percentage: 0.9, Color: "#EC4899"},
				{SVGIDs: []string{"gluteus_maximus", "gluteus_medius"}, Percentage: 0.8, Color: "#D946EF"},
				{SVGIDs: []string{"gastrocnemius", "soleus"}, Percentage: 0.6, Color: "#7C3AED"},
			},
		},
		{
			Filename: "man-full-body.svg",
			Gender:   "man",
			Scores: []muscle_heatmap_image.MuscleScore{
				{SVGIDs: []string{"trapezius"}, Percentage: 0.9, Color: "#EC4899"},
				{SVGIDs: []string{"latissimus_dorsi"}, Percentage: 0.8, Color: "#D946EF"},
				{SVGIDs: []string{"biceps"}, Percentage: 0.7, Color: "#7C3AED"},
				{SVGIDs: []string{"quadriceps"}, Percentage: 0.6, Color: "#8B5CF6"},
				{SVGIDs: []string{"abdominals"}, Percentage: 0.5, Color: "#9370DB"},
			},
		},
	}

	for _, s := range scenarios {
		fmt.Printf("Generating %s...\n", s.Filename)
		svg, err := provider.GenerateSVG(s.Gender, s.Scores)
		if err != nil {
			fmt.Printf("Error generating %s: %v\n", s.Filename, err)
			continue
		}

		path := filepath.Join(outputDir, s.Filename)
		if err := os.WriteFile(path, []byte(svg), 0644); err != nil {
			fmt.Printf("Error writing %s: %v\n", path, err)
		}
	}

	fmt.Println("Done! Examples generated in", outputDir)
}
