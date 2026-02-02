# Plugin System Architecture

FitGlue uses a **type-safe, self-registering plugin architecture** for extensible data processing.

> [!IMPORTANT]
> The **Plugin Registry** (`src/typescript/shared/src/plugin/registry.ts`) is the single source of truth for all plugin manifests. Configuration is served dynamically via `GET /api/registry`.

## Plugin Types

| Type | Language | Purpose | Examples |
|------|----------|---------|----------|
| **Source** | TypeScript | Ingests data from external services | Hevy, Fitbit, Strava, Polar, Oura, Wahoo |
| **Enricher** | Go | Transforms/enhances activities in pipeline | HR Summary, Weather, Parkrun, AI Companion |
| **Destination** | Go | Uploads processed activities | Strava, TrainingPeaks, Intervals.icu, Hevy |
| **Integration** | TypeScript | OAuth connection definitions | All OAuth providers |

## Architecture Overview

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Sources   │────▶│   Enrichers  │────▶│  Destinations   │
│ (TypeScript)│     │    (Go)      │     │     (Go)        │
└─────────────┘     └──────────────┘     └─────────────────┘
       │                   │                     │
       └───────────────────┼─────────────────────┘
                           ▼
                 ┌──────────────────┐
                 │  Plugin Registry │
                 │  GET /api/registry│
                 └──────────────────┘
```

## Available Plugins

### Sources (Data Ingestion)
| Source | Auth Type | Features |
|--------|-----------|----------|
| Hevy | API Key | Strength workouts, webhook sync |
| Fitbit | OAuth | Activity notifications, polling |
| Strava | OAuth | Activity webhooks |
| Polar | OAuth | Activity webhooks |
| Oura | OAuth | Sleep, readiness data |
| Wahoo | OAuth | Cycling/running data |
| Apple Health | Mobile Push | iOS health data |
| Health Connect | Mobile Push | Android health data |
| FIT Upload | Direct | Manual FIT file upload |
| Parkrun Results | Public ID | Race results via athlete ID |

### Enrichers (Pipeline Steps)

| Category | Enrichers |
|----------|-----------|
| **Data** | Fitbit HR, FIT File HR, Spotify Tracks, Weather, Running Dynamics |
| **Stats** | Heart Rate Summary, Pace Summary, Speed Summary, Power Summary, Cadence Summary, Elevation Summary, Training Load (TRIMP), Personal Records |
| **Visual** | Muscle Heatmap, Muscle Heatmap Image, Route Thumbnail |
| **Detection** | Parkrun Detector, Location Naming, Condition Matcher |
| **Transform** | Type Mapper, Auto Increment, Logic Gate, Activity Filter |
| **Input** | User Input, Hybrid Race Tagger |
| **AI** | AI Companion, AI Banner |

### Destinations (Data Export)
| Destination | Auth Type | Features |
|-------------|-----------|----------|
| Strava | OAuth | FIT upload, title/description sync |
| TrainingPeaks | OAuth | Workout upload |
| Intervals.icu | API Key | Activity upload |
| Hevy | API Key | Sync back to Hevy |
| Showcase | Built-in | Public activity sharing |

## Tier Restrictions

Some enrichers are restricted to higher tiers:

| Tier | Available Enrichers |
|------|---------------------|
| Free | Heart Rate Summary, Elevation, Type Mapper, User Input |
| Pro | All Free + Weather, Location, Training Load, Personal Records |
| Athlete | All Pro + AI Companion, AI Banner, Spotify Tracks |

## Registration Patterns

### Go Enrichers (Self-Registration)

Enrichers register themselves via `init()`:

```go
// functions/enricher/providers/weather.go
func init() {
    plugin.RegisterManifest(pb.EnricherProviderType_ENRICHER_PROVIDER_WEATHER, &pb.PluginManifest{
        Id:          "weather",
        Name:        "Weather",
        Description: "Adds weather conditions based on activity location and time",
        Icon:        "🌤️",
        Tier:        pb.UserTier_USER_TIER_PRO,  // Tier restriction
        ConfigSchema: []*pb.ConfigFieldSchema{
            {Key: "units", Label: "Units", FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT},
        },
    })
    Register(NewWeatherProvider())
}
```

### TypeScript Sources & Destinations

Sources and destinations register in `shared/src/plugin/registry.ts`:

```typescript
registerSource({
  id: 'hevy',
  name: 'Hevy',
  icon: '💪',
  enabled: true,
  authType: AuthenticationType.AUTHENTICATION_TYPE_API_KEY,
  marketingDescription: `
    ### Strength Training Source
    Import your weight training workouts from Hevy...
  `,
  features: [
    '✅ Import strength workouts with full exercise details',
    '✅ Real-time sync via webhooks',
  ],
  externalUrlTemplate: 'https://hevy.com/workout/{externalActivityId}',
});
```

## Configuration Schema

Plugins define their configuration using `ConfigFieldSchema`:

| Field Type | Description | Example |
|------------|-------------|---------|
| `STRING` | Text input | API keys, names |
| `NUMBER` | Numeric input | Timeout values |
| `BOOLEAN` | Toggle | Enable/disable features |
| `SELECT` | Dropdown | Format options |
| `MULTI_SELECT` | Multi-choice | Days of week |
| `KEY_VALUE_MAP` | Key-value pairs | Type mappings |
| `ACTIVITY_TYPE_SELECT` | Activity type picker | Filter by type |

## Discovery API

The plugin registry is exposed via:

```
GET /api/registry?marketingMode=false
```

**Parameters:**
- `marketingMode=true`: Include marketing descriptions for web pages
- `marketingMode=false`: Minimal data for app UI

**Returns:**
```json
{
  "sources": [...],
  "enrichers": [...],
  "destinations": [...],
  "integrations": [...]
}
```

Used by the frontend to dynamically render plugin selection and configuration forms.

---

## Scaffolding New Plugins

FitGlue provides scaffolding commands to minimize boilerplate when adding new plugins.

### Quick Start

```bash
# Add a new data source (TypeScript webhook handler)
make plugin-source name=garmin

# Add a new enricher (Go pipeline step)
make plugin-enricher name=weather

# Add a new destination (Go uploader)
make plugin-destination name=runkeeper
```

### What Gets Generated

#### Source (`make plugin-source name=NAME`)

| Generated | Location |
|-----------|----------|
| Handler directory | `src/typescript/{name}-handler/` |
| package.json | Standard dependencies |
| Connector class | `src/connector.ts` with TODO markers |
| Terraform config | Appended to `functions.tf` |
| index.js export | Appended automatically |

**Remaining manual steps:**
1. Add `CloudEventSource.CLOUD_EVENT_SOURCE_{NAME}` to `events.proto`
2. Add `ActivitySource.SOURCE_{NAME}` to `activity.proto`
3. Run `make generate`
4. Add Firebase rewrite to `web/firebase.json`
5. Register in `shared/src/plugin/registry.ts`
6. Implement the connector logic
7. **Update documentation** (`docs/architecture/plugin-system.md`)

#### Enricher (`make plugin-enricher name=NAME`)

| Generated | Location |
|-----------|----------|
| Provider file | `src/go/functions/enricher/providers/{name}.go` |
| Proto enum | Auto-added to `user.proto` |
| Type regeneration | Runs `make generate` automatically |

**Remaining manual steps:**
1. Implement the `Enrich()` method
2. Add config fields to the manifest
3. **Update documentation** (`docs/architecture/plugin-system.md`)

#### Destination (`make plugin-destination name=NAME`)

| Generated | Location |
|-----------|----------|
| Uploader function | `src/go/functions/{name}-uploader/` |
| Proto enum | Auto-added to `events.proto` |
| Terraform config | Appended to `functions.tf` |
| Type regeneration | Runs `make generate` automatically |

**Remaining manual steps:**
1. Implement upload logic
2. Add routing case in `router/function.go`
3. Register in `shared/src/plugin/registry.ts`
4. **Update documentation** (`docs/architecture/plugin-system.md`)

### Example: Adding a Weather Enricher

```bash
make plugin-enricher name=weather
```

Output:
```
Creating enricher: weather
  Using enum value: 13
✓ Added ENRICHER_PROVIDER_WEATHER = 13 to user.proto
✓ Created src/go/functions/enricher/providers/weather.go
Running 'make generate' to regenerate types...
✓ Enricher scaffolding complete!

Next steps:
  1. Implement the Enrich() method
  2. Add config fields to the manifest if needed
  3. Run 'make test-go' to verify
  4. Update docs/architecture/plugin-system.md
```

### Naming Conventions

| Input | Generated Names |
|-------|-----------------|
| `garmin` | `GarminConnector`, `garmin-handler`, `GARMIN` |
| `heart_rate` | `HeartRateProvider`, `heart_rate.go`, `HEART_RATE` |

Use lowercase with underscores. The script handles case conversion.

---

## Related Documentation

- [Registry Reference](../reference/registry.md) - API and manifest structure
- [Architecture Overview](overview.md) - System components
- [Enricher Testing](../guides/enricher-testing.md) - Testing enrichers
