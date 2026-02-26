# FIT File Generation & Testing

This document describes the tools and processes for generating FIT files used in verification and testing, particularly for Strava uploads.

## FIT Generator Tool (`fit-gen`)

The `fit-gen` CLI tool (`src/go/cmd/fit-gen`) converts a `StandardizedActivity` JSON representation into a valid binary `.fit` file.

### Build
```bash
make build-go
# Binary location: ./bin/fit-gen
```

### Usage
```bash
./bin/fit-gen -input <path-to-json-activity> -output <path-to-fit-file>
```

## Test Data Stubs

Located in `src/go/cmd/fit-gen/stubs/`, these JSON files represent various activity scenarios (e.g., Weight Training, Running with GPS, Cycling with Power).

### Generating Stubs
A Python script is provided to generate realistic, 5-minute long activity stubs with staggered dates to avoid overlap.

**Script:** `src/go/cmd/fit-gen/stubs/generate_test_data.py`

**Usage:**
```bash
python3 src/go/cmd/fit-gen/stubs/generate_test_data.py
```
This command will regenerate the JSON stub files in the same directory.

## Validation Workflow

To manually verify FIT file correctness (e.g., for Strava):

1.  **Generate Stubs:** Run the python script to get fresh data.
2.  **Generate FIT Files:** Use `fit-gen` to convert the JSON stubs to `.fit` files.
    ```bash
    ./bin/fit-gen -input src/go/cmd/fit-gen/stubs/verify_run_gps_hr.json -output src/go/verify_run_gps_hr.fit
    ```
3.  **Upload:** Upload the resulting `.fit` file to Strava (or other platform).
4.  **Verify:** Check that all data fields (Heart Rate, GPS map, Power, etc.) are displayed correctly.

## FIT Inspector Tool (`fit-inspect`)

The `fit-inspect` CLI tool (`src/go/cmd/fit-inspect`) provides a quick way to analyze the contents of a FIT file without uploading it to a third-party service. It calculates statistics for key metrics and can dump raw record data.

### Build
```bash
make build-go
# Binary location: ./bin/fit-inspect
```

### Usage
```bash
./bin/fit-inspect -input <path-to-fit-file> [flags]
```

**Flags:**
- `-input`: (Required) Path to the FIT file to analyze.
- `-detailed-dump`: (Optional) If set, prints every record's raw field values and types to stdout. Useful for debugging field name mismatches or data issues.

### Output
The tool outputs a statistical summary table for the following fields (if present):
- HeartRate
- Power
- Cadence
- Speed
- Distance
- Altitude
- PositionLat
- PositionLong

**Example Output:**
```text
Analyzing FIT file...

Total Records: 300

Field Statistics:
Field           Count   Coverage   Min              Max              Avg
-----           -----   --------   ---              ---              ---
heart_rate      300     100.0%     121.00           159.00           140.56
power           300     100.0%     200.00           250.00           225.00
...
```

## FIT Combiner Tool (`fit-combine`)

The `fit-combine` CLI tool (`src/go/cmd/fit-combine`) merges two FIT files into a single output FIT file. Records are sorted by timestamp, laps and sessions are re-indexed, and the result contains a single FileId and Activity message.

### Build
```bash
make build-tools-go
# Binary location: ./bin/fit-combine
```

### Usage
```bash
./bin/fit-combine -input1 <file1.fit> -input2 <file2.fit> [-output <combined.fit>]
```

**Flags:**
- `-input1`: (Required) Path to the first FIT file.
- `-input2`: (Required) Path to the second FIT file.
- `-output`: (Optional, default `combined.fit`) Path to write the merged FIT file.

### Merge Strategy
- **FileId & DeviceInfo**: Taken from the first file only.
- **Records**: Combined from both files, sorted by timestamp.
- **Laps & Sessions**: Combined, sorted by start time, and message indices re-numbered.
- **Sets**: Combined, sorted by timestamp, re-indexed.
- **Activity**: Single Activity message with the combined session count.

### Example
```bash
./bin/fit-combine \
  -input1 src/go/cmd/fit-inspect/examples/parkrun.fit \
  -input2 src/go/cmd/fit-inspect/examples/sprints.fit \
  -output /tmp/combined.fit

# Verify the result
./bin/fit-inspect -input /tmp/combined.fit
```
