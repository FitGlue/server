package muscle_heatmap_image

// BodySVGTemplate is the base SVG with named muscle paths
// Each muscle group has a unique ID that can be styled with fill colors
const BodySVGTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 600" width="400" height="600">
  <defs>
    <style>
      .muscle { stroke: #222; stroke-width: 1; }
      .outline { fill: none; stroke: #666; stroke-width: 2; }
    </style>
  </defs>

  <!-- Body Outline -->
  <path class="outline" d="M 200 50 L 180 80 L 170 120 L 165 180 L 160 250 L 155 350 L 150 450 L 145 550 M 200 50 L 220 80 L 230 120 L 235 180 L 240 250 L 245 350 L 250 450 L 255 550 M 145 550 L 140 580 M 255 550 L 260 580 M 170 120 L 130 140 L 110 160 M 230 120 L 270 140 L 290 160"/>

  <!-- Front View Muscles -->

  <!-- Chest (Pectorals) -->
  <path id="chest" class="muscle" fill="{{.chest}}" d="M 180 120 Q 200 110 220 120 Q 210 140 200 145 Q 190 140 180 120 Z"/>

  <!-- Shoulders (Deltoids) -->
  <path id="shoulders" class="muscle" fill="{{.shoulders}}" d="M 170 100 Q 165 110 170 120 L 180 115 L 175 100 Z M 230 100 Q 235 110 230 120 L 220 115 L 225 100 Z"/>

  <!-- Biceps -->
  <path id="biceps" class="muscle" fill="{{.biceps}}" d="M 170 140 Q 165 160 168 180 L 175 175 L 172 140 Z M 230 140 Q 235 160 232 180 L 225 175 L 228 140 Z"/>

  <!-- Forearms -->
  <path id="forearms" class="muscle" fill="{{.forearms}}" d="M 168 190 Q 165 220 162 250 L 168 248 L 170 190 Z M 232 190 Q 235 220 238 250 L 232 248 L 230 190 Z"/>

  <!-- Abs (Abdominals) -->
  <path id="abs" class="muscle" fill="{{.abs}}" d="M 185 150 L 215 150 L 218 200 L 182 200 Z"/>

  <!-- Obliques -->
  <path id="obliques" class="muscle" fill="{{.obliques}}" d="M 175 160 Q 170 180 172 200 L 180 195 L 178 160 Z M 225 160 Q 230 180 228 200 L 220 195 L 222 160 Z"/>

  <!-- Quads (Quadriceps) -->
  <path id="quads" class="muscle" fill="{{.quads}}" d="M 180 260 Q 175 310 172 360 L 178 358 L 182 260 Z M 220 260 Q 225 310 228 360 L 222 358 L 218 260 Z"/>

  <!-- Calves -->
  <path id="calves" class="muscle" fill="{{.calves}}" d="M 172 450 Q 170 490 168 530 L 174 528 L 176 450 Z M 228 450 Q 230 490 232 530 L 226 528 L 224 450 Z"/>

  <!-- Back View Muscles (offset to the right) -->

  <!-- Traps (Trapezius) -->
  <path id="traps" class="muscle" fill="{{.traps}}" d="M 320 90 Q 330 100 325 115 L 315 110 L 318 90 Z M 360 90 Q 350 100 355 115 L 365 110 L 362 90 Z"/>

  <!-- Lats (Latissimus Dorsi) -->
  <path id="lats" class="muscle" fill="{{.lats}}" d="M 310 130 Q 300 160 305 190 L 315 185 L 312 130 Z M 370 130 Q 380 160 375 190 L 365 185 L 368 130 Z"/>

  <!-- Lower Back -->
  <path id="lower_back" class="muscle" fill="{{.lower_back}}" d="M 325 200 L 355 200 L 352 240 L 328 240 Z"/>

  <!-- Glutes -->
  <path id="glutes" class="muscle" fill="{{.glutes}}" d="M 320 250 Q 315 270 318 290 L 325 288 L 322 250 Z M 360 250 Q 365 270 362 290 L 355 288 L 358 250 Z"/>

  <!-- Hamstrings -->
  <path id="hamstrings" class="muscle" fill="{{.hamstrings}}" d="M 318 300 Q 313 350 310 400 L 316 398 L 320 300 Z M 362 300 Q 367 350 370 400 L 364 398 L 360 300 Z"/>

  <!-- Triceps (back of arm) -->
  <path id="triceps" class="muscle" fill="{{.triceps}}" d="M 310 140 Q 305 160 308 180 L 314 178 L 312 140 Z M 370 140 Q 375 160 372 180 L 366 178 L 368 140 Z"/>

  <!-- Upper Back -->
  <path id="upper_back" class="muscle" fill="{{.upper_back}}" d="M 325 120 L 355 120 L 352 160 L 328 160 Z"/>
</svg>`

// MusclePathIDs maps muscle group names to SVG path IDs
var MusclePathIDs = map[string]string{
	"chest":      "chest",
	"shoulders":  "shoulders",
	"biceps":     "biceps",
	"triceps":    "triceps",
	"forearms":   "forearms",
	"lats":       "lats",
	"traps":      "traps",
	"upper_back": "upper_back",
	"lower_back": "lower_back",
	"abs":        "abs",
	"obliques":   "obliques",
	"quads":      "quads",
	"hamstrings": "hamstrings",
	"glutes":     "glutes",
	"calves":     "calves",
}
