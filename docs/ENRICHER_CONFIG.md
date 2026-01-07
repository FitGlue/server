# Enricher Configuration Guide

This guide explains how to configure enrichment pipelines for users in FitGlue.

## Overview

Enrichers are modular providers that transform and enhance standardized activities before they are sent to destinations. Each user can have multiple pipelines, and each pipeline can have multiple enrichers that execute in sequence.

## Pipeline Architecture

```
Source (Hevy) → [Enricher 1] → [Enricher 2] → [Enricher N] → Destination (Strava)
```

**Key Concepts**:
- **Source**: Where the activity originates (e.g., `SOURCE_HEVY`)
- **Enrichers**: Ordered list of providers that process the activity
- **Destinations**: Where the enriched activity is sent (e.g., `strava`)

## Available Enrichers

### 1. Metadata Passthrough
**Provider Type**: `ENRICHER_PROVIDER_METADATA_PASSTHROUGH`

**Purpose**: Preserves the original activity name and description from the source.

**When to Use**: First in pipeline to seed metadata before other enrichers append to it.

**Configuration**: None required
```json
{
  "providerType": 5,
  "inputs": {}
}
```

**Output**: Sets `name` and `description` fields on the enriched activity.

---

### 2. Workout Summary
**Provider Type**: `ENRICHER_PROVIDER_WORKOUT_SUMMARY`

**Purpose**: Generates a formatted summary of strength training exercises.

**Trigger**: Activity contains `StrengthSets`

**Configuration**:
```json
{
  "providerType": 2,
  "inputs": {
    "format": "detailed", // Options: "compact", "detailed", "verbose"
    "show_stats": "true"  // Show headline stats (total volume, sets, etc.) - default: true
  }
}
```

**Format Styles**:
- **Compact**: `4×8@100kg` - Minimal spacing, no decimals
- **Detailed** (default): `4 sets × 8 reps @ 100.0kg` - Readable with units
- **Verbose**: `4 sets of 8 reps at 100.0 kilograms` - Full words

**Headline Stats**:
When `show_stats` is true, a summary line is added at the top:
`📊 12 sets • 4,200kg volume • 120 reps • Heaviest: 140kg (Squat)`

**Output Example** (Detailed):
```
Workout Summary:
📊 12 sets • 4,200kg volume • 120 reps

- Bench Press: 4 x 8 × 100.0kg
- Squats: 3 x 10 × 120.0kg
- Deadlifts: 5 x 5 × 140.0kg
```

**Appends to**: `description` field

---

### *. Type Mapper
**Provider Type**: `ENRICHER_PROVIDER_TYPE_MAPPER`

**Purpose**: Overrides the FIT file activity type based on text matching rules in the activity name.

**Trigger**: Activity name matches configured rules.

**Configuration**:
```json
{
  "providerType": 7,
  "inputs": {
    "rules": "[{\"substring\": \"Yoga\", \"target_type\": \"YOGA\"}, {\"substring\": \"Run\", \"target_type\": \"RUNNING\"}]"
  }
}
```

**Rule Logic**:
- Checks config rules in order.
- If `activity.Name` contains `rule.substring` (case-insensitive):
  - Sets `activity.Type` to `rule.target_type`.
  - Stops at first match.

**Supported Target Types**:
`WEIGHT_TRAINING`, `RUNNING`, `CYCLING`, `SWIMMING`, `HIKING`, `WALKING`, `YOGA`, `WORKOUT`, `CROSSFIT`

**Output**: Modifies `type` field of the activity (does not append to description).

---

### 3. Muscle Heatmap
**Provider Type**: `ENRICHER_PROVIDER_MUSCLE_HEATMAP`

**Purpose**: Visualizes muscle activation using emoji bar charts.

**Trigger**: Activity contains `StrengthSets` with muscle group data

**Configuration**: None required
```json
{
  "providerType": 3,
  "inputs": {}
}
```

**Output Example**:
```
💪 Muscle Activation:
Chest: 🟪🟪🟪🟪⬜
Quadriceps: 🟪🟪🟪⬜⬜
Lower Back: 🟪🟪🟪🟪🟪
```

**Appends to**: `description` field

**Coefficients** (hardcoded):
- Large muscles (Quadriceps, Chest, Lats): 1.0
- Medium muscles (Shoulders, Biceps, Triceps): 0.7
- Small muscles (Calves, Forearms): 0.5

---

### 4. Virtual GPS
**Provider Type**: `ENRICHER_PROVIDER_VIRTUAL_GPS`

**Purpose**: Generates synthetic GPS tracks for indoor/non-GPS activities.

**Trigger**: `TotalDistance > 0` AND no existing GPS data

**Configuration**:
```json
{
  "providerType": 6,
  "inputs": {
    "route": "london",  // Optional: route name (default: "london")
    "force": "true"     // Optional: overwrite existing GPS (default: false)
  }
}
```

**Available Routes**:
- `london`: London Hyde Park loop (~4km)
- *(Future: `nyc`, `paris`, etc.)*

**Output**: Injects `PositionLatStream` and `PositionLongStream` into activity records.

**How it Works**:
1. Calculates average speed: `distance / duration`
2. Projects distance onto route coordinates
3. Loops route if distance > route length
4. Interpolates timestamps at 1Hz

---

### 5. Source Link
**Provider Type**: `ENRICHER_PROVIDER_SOURCE_LINK`

**Purpose**: Adds a link back to the original workout in the source app.

**Trigger**: Activity has `ExternalId`

**Configuration**: None required
```json
{
  "providerType": 4,
  "inputs": {}
}
```

**Output Example**:
```
View on Hevy: https://hevy.com/workout/abc123
```

**Appends to**: `description` field

---

### 6. Fitbit Heart Rate
**Provider Type**: `ENRICHER_PROVIDER_FITBIT_HEART_RATE`

**Purpose**: Fetches heart rate data from Fitbit API and merges into activity.

**Trigger**: User has Fitbit integration enabled

**Configuration**:
```json
{
  "providerType": 1,
  "inputs": {}
}
```

**Requirements**:
- User must have authorized Fitbit via OAuth
- Fitbit account must have HR data for workout time range

**Output**: Injects `HeartRateStream` into activity records.

**API Call**: Fetches intraday HR data for workout start time + duration.

---

### 7. Parkrun
**Provider Type**: `ENRICHER_PROVIDER_PARKRUN`

**Purpose**: Identifies Parkrun activities, titles them correctly (e.g. "Bushy Parkrun"), and adds tags.

**Trigger**: Activity is a Run + Sat 9am + Near known Parkrun location.

**Configuration**:
```json
{
  "providerType": 8,
  "inputs": {
    "enable_titling": "true", // Rename activity to event name (default: true)
    "tags": "Parkrun, Race"   // Comma-separated tags (default: "Parkrun")
  }
}
```

**Output**: Sets `name` and adds `tags`.

**How it Works**:
Checks start location against known Parkrun coordinates. If within 200m and time matches Saturday morning window, it applies the event name and tags.

---

### 8. Condition Matcher
**Provider Type**: `ENRICHER_PROVIDER_CONDITION_MATCHER`

**Purpose**: Apply changes (Title, Description) ONLY if specific conditions are met (Time, Location, Type).

**Trigger**: Configurable conditions.

**Configuration**:
```json
{
  "providerType": 9,
  "inputs": {
    "activity_type": "RUNNING",       // Match specific type
    "days": "Sat,Sun",                // Match days of week
    "start_time": "06:00",            // Match start time >= 06:00
    "end_time": "10:00",              // Match start time <= 10:00
    "location_lat": "51.456",         // Target Latitude
    "location_long": "-0.123",        // Target Longitude
    "radius_m": "500",                // Radius in meters
    "title_template": "Morning Run",  // Apply if matched
    "description_template": "Matched condition!" // Apply if matched
  }
}
```

**Output**: Conditionally sets `name` or `description`.

---

### 9. Auto Increment
**Provider Type**: `ENRICHER_PROVIDER_AUTO_INCREMENT`

**Purpose**: Maintains a counter and appends the current count to the activity name (e.g., "Run (#100)").

**Trigger**: Always runs if configured.

**Configuration**:
```json
{
  "providerType": 10,
  "inputs": {
    "counter_key": "my_run_counter", // Unique ID for this counter
    "initial_value": "1",            // Start counting from here
    "title_contains": "Run"          // Optional: Only increment if title contains this
  }
}
```

**Output**: Appends `(#N)` to the activity name.

---

### 10. User Input
**Provider Type**: `ENRICHER_PROVIDER_USER_INPUT`

**Purpose**: Pauses the pipeline and requests manual input from the user via the CLI (or future UI).

**Trigger**: Always runs.

**Configuration**:
```json
{
  "providerType": 11,
  "inputs": {
    "fields": "title,description" // Fields to request
  }
}
```

**Behavior**:
1. Pipeline halts with status `STATUS_WAITING`.
2. Admin/User must use CLI `inputs:resolve` to provide data.
3. Pipeline resumes and injects input into activity.

---

### 11. Activity Filter
**Provider Type**: `ENRICHER_PROVIDER_ACTIVITY_FILTER`

**Purpose**: Stops pipeline execution for specific activities based on type or content analysis.

**Trigger**: Always runs if configured.

**Configuration**:
```json
{
  "providerType": 12,
  "inputs": {
    "exclude_activity_types": "WALK,YOGA", // Comma-separated list of types to skip
    "exclude_title_contains": "commute",   // Skip if title contains this (case-insensitive)
    "exclude_description_contains": "",    // Skip if description contains this
    "include_activity_types": "RUNNING",   // OPTIONAL: Only allow these types
    "include_title_contains": "",          // OPTIONAL: Only allow if title contains this
    "include_description_contains": ""     // OPTIONAL: Only allow if description contains this
  }
}
```

**Behavior**:
1. **Exclusions**: Checks if activity matches any `exclude_*` criteria. If matched -> **HALT**.
2. **Inclusions** (Allow-list):
   - If any `include_*` fields are set, activity MUST match at least one of them.
   - If no match found -> **HALT**.
   - If no `include_*` fields are set -> **PASS**.
   - Matches are case-insensitive.
- When halted, Orchestrator returns `STATUS_SKIPPED`.

## Configuring Pipelines via Admin CLI

### Add a Pipeline

```bash
./fitglue-admin users:add-pipeline <user-id>
```

**Interactive Prompts**:
1. **Source**: Select activity source (e.g., `SOURCE_HEVY`)
2. **Enrichers**: Add enrichers in desired order
   - Select provider type from list
   - Enter configuration JSON (optional)
   - Repeat to add more enrichers
3. **Destinations**: Select where to send enriched activity (e.g., `strava`)

**Example Session**:
```
? Select Source: SOURCE_HEVY
? Add an enricher? Yes
? Enricher Provider: Metadata Passthrough
? Inputs (JSON string, optional): <Enter>
? Add an enricher? Yes
? Enricher Provider: Workout Summary
? Inputs (JSON string, optional): <Enter>
? Add an enricher? Yes
? Enricher Provider: Virtual GPS
? Inputs (JSON string, optional): {"route": "london"}
? Add an enricher? No
? Select Destinations: strava
Pipeline added successfully! ID: pipe-abc123
```

### Remove a Pipeline

```bash
./fitglue-admin users:remove-pipeline <user-id>
```

Interactively select pipeline to delete.

### Replace a Pipeline

```bash
./fitglue-admin users:replace-pipeline <user-id>
```

Interactively select pipeline and reconfigure (keeps same ID).

---

## Best Practices

### Enricher Order

**Recommended Order**:
1. **Workout Summary** - Adds exercise details
2. **Muscle Heatmap** - Adds muscle visualization
3. **Fitbit Heart Rate** - Adds HR data
4. **Virtual GPS** - Adds location data (if needed)
5. **Source Link** - Adds source URL at end

**Why?** Description-based enrichers append text sequentially. Placing Source Link last ensures the URL appears at the bottom of the description.

### Multiple Pipelines

You can create multiple pipelines for the same source:

**Use Cases**:
- **Production + Test**: One with full enrichment, one minimal for testing
- **Different Destinations**: One for Strava, one for future integrations
- **A/B Testing**: Compare enrichment strategies

**Example**:
```bash
# Pipeline 1: Full enrichment for Strava
./fitglue-admin users:add-pipeline user-123
# Enrichers: Metadata → Summary → Heatmap → Fitbit HR → Virtual GPS → Link
# Destinations: strava

# Pipeline 2: Minimal for backup
./fitglue-admin users:add-pipeline user-123
# Enrichers: Metadata Passthrough
# Destinations: strava
```

**Result**: Each Hevy workout creates TWO Strava activities.

### Configuration Tips

1. **Start Simple**: Begin with Metadata Passthrough only, then add enrichers incrementally
2. **Test Locally**: Use test activities to verify enricher output before production
3. **Monitor Logs**: Check execution logs to verify enrichers are running
4. **Validate FIT Files**: Use `fit-inspect` to verify binary output

---

## Troubleshooting

### "Provider not found, skipping"
**Cause**: Enricher not registered in `enricher/function.go`
**Solution**: Verify provider is in `providerRegistry` array

### Virtual GPS Not Generating Coordinates
**Cause**: Activity has `TotalDistance = 0` or existing GPS data
**Solution**: Check activity data, use `"force": "true"` config to override

### Muscle Heatmap Empty
**Cause**: No `StrengthSets` in activity or missing muscle group data
**Solution**: Ensure source (Hevy) provides muscle group information

### Fitbit HR Failed
**Cause**: OAuth tokens expired or Fitbit integration not enabled
**Solution**: Re-authorize via `./fitglue-admin users:connect <user-id> fitbit`

### Description Not Concatenated
**Cause**: Enrichers running in wrong order or merge logic issue
**Solution**: Check enricher order in pipeline configuration

---

## Advanced: Provider Inputs

Some enrichers support configuration via the `inputs` field:

### Virtual GPS Inputs
```json
{
  "route": "london",    // Route selection
  "force": "true"       // Overwrite existing GPS
}
```

### Future: Configurable Providers
Planned enhancements:
- **Workout Summary**: Custom formatting templates
- **Muscle Heatmap**: Adjustable coefficients
- **Fitbit HR**: Priority levels for data fetching

---

## See Also

- [Enricher Testing Guide](ENRICHER_TESTING.md) - Comprehensive testing procedures
- [Admin CLI Guide](ADMIN_CLI.md) - Full CLI command reference
- [Manual QA Guide](MANUAL_QA_GUIDE.md) - End-to-end testing workflows
