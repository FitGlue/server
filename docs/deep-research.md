# **Cloud-Native Architecture for Unified Fitness Telemetry: Integrating Hevy, Keiser, and Fitbit with Strava on Google Cloud Platform**

## **1\. Architectural Vision and Infrastructure Design**

The modern fitness data landscape is characterized by a fragmented ecosystem where specialized platforms—Hevy for strength training, Keiser for power-based indoor cycling, and Fitbit for continuous biometric monitoring—operate in siloed environments. For the software engineer acting as a "Quantified Self" architect, the objective is not merely synchronization but the synthesis of these disparate data streams into a cohesive, enriched narrative within Strava. This report outlines a comprehensive, cloud-native architecture on Google Cloud Platform (GCP) designed to ingest, normalize, merge, and enhance this data.

The proposed solution adheres to a **Serverless Event-Driven Microservices** pattern. This choice is predicated on the intermittent nature of fitness data (bursts of activity followed by dormancy) and the requirement for a "cloud-only" footprint, eliminating the operational overhead of managing persistent virtual machines. By leveraging Google Cloud Functions (2nd Gen), Cloud Pub/Sub, and Cloud Firestore, the architecture achieves a high degree of scalability and cost-efficiency while supporting the polyglot requirements of the user: TypeScript for SDK-heavy integrations (Keiser, Hevy) and Golang for high-performance data manipulation and binary file generation.

### **1.1 The Integration Landscape and Data Lakehouse Strategy**

The architecture functions as a specialized "Fitness Data Lakehouse," creating a central repository where raw telemetry is stored, processed, and refined before egress to the downstream consumer (Strava). The complexity of this integration stems from the heterogeneity of the source systems:

* **Hevy:** Operates primarily via a webhook-driven model for completed workouts, providing JSON payloads structured around sets, repetitions, and weight volume.1
* **Keiser M Series:** Relies on a cloud-synchronization model where the machine (M3i) communicates with a mobile app, which then pushes to the Keiser Cloud. Accessing this requires interfacing with the Keiser Metrics SDK, specifically the TypeScript implementation, to retrieve session data sets containing power and cadence streams.3
* **Fitbit:** Serves as the biometric master, providing heart rate data. Crucially, the requirement for precise data merging necessitates access to the Intraday Heart Rate Time Series API, which offers 1-second granularity, distinct from the standard minute-level summaries.4

The architectural challenge is orchestrating these sources such that a "Ride" recorded on a Keiser bike is not merely uploaded as a summary but is programmatically merged with the concurrent heart rate stream from Fitbit to produce a single, unified .fit file for Strava. Simultaneously, strength training sessions from Hevy must be enriched with visualizations (heatmaps) and descriptive metadata that the native integration lacks.

### **1.2 Google Cloud Platform Component Architecture**

The infrastructure is composed of the following managed services, selected to optimize for the specific behaviors of the external APIs involved:

* **Cloud Functions (Gen 2):** Serving as the compute layer, these functions host the ingestion and processing logic. Gen 2 functions are built on Cloud Run, offering longer execution times (critical for polling loops and large file generation) and higher concurrency.
  * *Ingestion Functions:* Node.js runtime is selected for the hevy-webhook-handler and keiser-poller to utilize the native TypeScript SDKs provided by these platforms.
  * *Processing Functions:* The Go 1.21+ runtime is selected for the activity-enhancer and fit-file-generator services. Go’s strong typing and superior performance in handling binary streams make it the ideal choice for constructing Flexible and Interoperable Data Transfer (FIT) files.
* **Cloud Pub/Sub:** Acts as the asynchronous messaging backbone. When an ingestion function detects a new activity (via webhook or poll), it publishes a message to a specific topic (e.g., topic-new-activity-hevy or topic-new-activity-keiser). This decouples the ingestion from the processing, ensuring that rate limits on the egress side (Strava API) do not cause upstream failures.
* **Cloud Firestore:** Implements the state management layer. It stores:
  * **OAuth State:** Access and Refresh tokens for Strava and Fitbit, indexed by user ID.
  * **Activity Cursor:** The timestamp of the last successfully synced Keiser session to prevent duplicate processing.
  * **Parkrun Geodata:** A cached collection of global Parkrun event locations for geospatial querying.
  * **Session State:** Temporary storage for "partial" activities waiting for their counterpart data (e.g., waiting for Fitbit sync to complete before merging with Keiser power data).
* **Secret Manager:** Provides the security envelope for sensitive credentials, including the Hevy Pro API Key, Keiser credentials, and OAuth Client Secrets for Strava and Fitbit.
* **Cloud Scheduler:** Configured to trigger the polling functions (Keiser and Fitbit) at regular intervals (e.g., every 15 minutes), as these platforms do not provide real-time webhooks for all data types.

## ---

**2\. Data Ingestion Layer: Protocols and Payload Analysis**

The Ingestion Layer is responsible for interfacing with the external APIs, validating data integrity, and normalizing the incoming telemetry into an internal canonical format stored in Firestore or passed via Pub/Sub.

### **2.1 Hevy Integration: Webhooks and Payload Parsing**

Hevy's integration model for Pro users centers on webhooks, which provide a push-based mechanism for workout completion events.

**Mechanism:** The hevy-webhook-handler Cloud Function exposes an HTTPS endpoint registered in the Hevy Developer settings. Upon receiving a POST request, the function first verifies the X-Hevy-Signature (if available) or API key header to ensure authenticity.

**Payload Structure Analysis:** The JSON payload delivered by Hevy is comprehensive. It typically includes a root workout object containing metadata (title, start\_time, end\_time, description) and an exercises array.1 Each element in the exercises array represents a distinct movement, further nested with a sets array. Key fields for extraction include:

* exercise\_template\_id: A unique identifier for the exercise type (e.g., "Bench Press").
* sets: An array of objects containing weight\_kg, reps, rpe (Rate of Perceived Exertion), and distance\_meters.
* notes: User-generated comments specific to the exercise.

**Operational Handling:** The handler parses this JSON and identifies the activity type. While Hevy natively syncs to Strava, the user's requirement is to *enhance* this data. Therefore, the strategy is to intercept the data *before* or *parallel to* the native sync. If Hevy's native sync is disabled (recommended to avoid duplicates), this function becomes the sole uploader. The handler extracts the exercise list and volumes (Weight × Reps) to prepare for the Description Automation and Heatmap Generation phases.

### **2.2 Keiser M Series: SDK Implementation and Polling**

Integrating Keiser requires a proactive polling approach using the Keiser.Metrics.SDK.SSO.Typescript. Unlike webhooks, the system must ask "What is new?"

**Authentication Flow:** The Keiser SDK utilizes a bearer token authentication scheme. The authLogin method 3 exchanges a username and password for a session token. To maintain a "cloud-only" persistence, the resulting refresh token is stored in Firestore. The keiser-poller function checks Firestore for a valid token; if expired, it utilizes authExchangeFulfillment 6 to renew the session without storing user credentials in plain text.

**Data Retrieval Strategy:** The polling function invokes the SDK to list sessions (MSeriesDataSetData or equivalent session list endpoint) occurring after the last\_sync\_timestamp retrieved from Firestore.

* **Model Analysis:** The MSeriesDataSetData 3 is the critical data structure. It contains arrays of telemetry points sampled at high frequency (typically 1Hz or 4Hz depending on the bike generation).
* **Fields of Interest:**
  * Power: The wattage output, critical for cyclists tracking Functional Threshold Power (FTP).
  * Cadence: RPM data.
  * Gear: The resistance gear level.
  * Energy: Accumulative caloric burn or kilojoules.
  * Duration: Total elapsed time.

**Latency Management:** Keiser data originates from a Bluetooth upload via a mobile device. This introduces a "sync gap" where a ride finished at 9:00 AM might not appear in the cloud until 9:05 AM. The Cloud Scheduler frequency (e.g., every 10-15 minutes) is tuned to balance API load against data freshness.

### **2.3 Fitbit Intraday API: Biometric Synchronization**

Fitbit acts as the "biometric clock," providing the Heart Rate (HR) stream that must be fused with the Keiser power stream.

**Authorization Constraints:** Accessing the Intraday Heart Rate series (1-second intervals) is restricted to the "Personal" OAuth application type.4 This distinction is vital during the Fitbit App registration process; selecting "Server" or "Client" types will result in permission errors when requesting the /1sec resource.

Endpoint Interaction: The integration targets the endpoint:
GET /1/user/\[user-id\]/activities/heart/date/\[date\]/1d/1sec.json.4
The response object activities-heart-intraday contains a dataset array where each entry holds a time (HH:MM:SS) and a value (BPM).
**Synchronization Logic:** This retrieval is triggered *on demand* by the processing layer. Once a Keiser activity is detected, the system calculates the start and end timestamps (in UTC) and requests the specific slice of HR data from Fitbit.

* **Wait Conditions:** Fitbit devices also suffer from sync latency. If the Keiser activity is found but the Fitbit API returns an empty dataset for that window, the system must trigger a "wait and retry" workflow (using Cloud Tasks or Pub/Sub delay) to allow the user's watch to sync with the Fitbit cloud.

## ---

**3\. The Transformation Engine: Data Fusion and Enhancement**

The core complexity of this architecture lies in the Transformation Layer, implemented as a Golang Cloud Function. This service receives raw inputs from the ingestion layer and produces a finalized, high-fidelity artifact.

### **3.1 High-Fidelity Data Merging (HR \+ Power)**

The primary engineering challenge is merging two asynchronous time-series datasets—Keiser Power and Fitbit Heart Rate—into a single synchronized file. Strava prefers the FIT (Flexible and Interoperable Data Transfer) file format for rich sensor data.7

**FIT File Anatomy:** A FIT file is a binary format consisting of a Header, a Data Records block, and a CRC. The Data Records block contains messages, most notably the Record message, which stores instantaneous telemetry.

* **Golang Implementation:** The system utilizes a Go library such as tormoder/fit or a custom binary writer to construct these messages.

**The Merging Algorithm:**

1. **Time Alignment:** Both data streams are converted to Unix Epoch timestamps. The Keiser stream acts as the "Master" clock because it defines the active pedaling time.
2. **Normalization:** The Keiser data is typically event-based (transmitting on change or fixed interval). It must be resampled to a strict 1Hz frequency to match standard FIT specifications for easy analysis.
3. **Data Fusion:** The algorithm iterates through the Keiser timestamps. For each second T:
   * Set Power \= Keiser Value.
   * Set Cadence \= Keiser Value.
   * *Lookup:* Query the Fitbit array for HR.
   * **Interpolation Strategy:** If Fitbit data is missing at T (packet loss or sampling gap), the system performs linear interpolation between HR and HR to maintain stream continuity. This prevents "dropouts" in the Strava analysis graphs.
4. **Serialization:** The fused record {Timestamp, Power, Cadence, HeartRate} is serialized into the binary FIT format and saved to a temporary location in Google Cloud Storage (GCS).

**Insight:** This merge is critical for calculating advanced metrics. Strava's "Relative Effort" relies on Heart Rate, while "Training Load" relies on Power. By merging them, the user gains a holistic view of cardiovascular cost versus mechanical output.

### **3.2 Automated Description Engineering**

Strava descriptions serve as the narrative layer for the activity. The system programmatically generates these descriptions to provide context that raw numbers cannot.

**Template System:** Using Golang's text/template package, the system populates a predefined structure based on the activity type.

* **For Cycling (Keiser):**
  * *Template:* "Keiser M3i Indoor Session\\n\\n📊 Stats:\\n- Avg Power: {{.AvgWatts}}W\\n- Normalized Power: {{.NormPower}}W\\n- Efficiency Factor: {{.EF}}\\n\\n❤️ HR Data synced from Fitbit."
  * *Calculation:* The enhancer calculates Normalized Power (NP) and Efficiency Factor (NP / Avg HR) internally before upload, as Strava computes these but does not display them prominently in the description.
* **For Strength (Hevy):**
  * *Template:* "🏋️ Strength Training\\n\\nfocus: {{.FocusGroups}}\\n\\nexercises:\\n{{range.Exercises}}- {{.Name}}: {{.Sets}} x {{.Reps}} @ {{.Weight}}kg\\n{{end}}"
  * *Logic:* The system aggregates the sets to avoid verbose spam (e.g., combining "3 sets of 10" into a single line) and identifies the primary muscle groups worked to populate the FocusGroups variable.

### **3.3 Parkrun Auto-Tagging: Geospatial Logic**

The requirement to auto-tag Parkruns necessitates a geospatial query against a known database of event locations.

**Data Source:** Parkrun event locations are exposed via events.json, a file utilized by their map infrastructure.9 This file contains a FeatureCollection of every event's coordinates.

* **Infrastructure:** A separate Cloud Function (or a startup routine in the Enhancer) fetches and caches this events.json into a Firestore collection with geospatial indexing enabled, or simply caches it in memory if the dataset is small enough (approx. 2000 events globally).

**The Detection Algorithm:**

1. **Trigger:** The logic activates only for activities with type="Run".
2. **Temporal Filter:** It checks if the start\_date\_local corresponds to a Saturday morning (typically 09:00 AM or 09:30 AM local time).
3. **Spatial Filter:** The system compares the activity's start\_latlng 11 against the cached Parkrun coordinates.
   * *Math:* It utilizes the Haversine formula 12 to calculate the great-circle distance between the user's start point and the event location.
   * *Threshold:* If Distance \< 200m, a match is declared.
4. **Tagging Action:** Upon a match:
   * **Title Update:** The activity name is prefixed with the Parkrun event name (e.g., "Bushy Parkrun").
   * **Activity Type:** The workout\_type field in the Strava payload is set to 1 (Race).13 This ensures the activity is categorized correctly in Strava's analysis views.

### **3.4 Muscle Heatmap Generation: The SVG Renderer**

Hevy provides the data, but not the visual "Muscle Heatmap" asset via API. To satisfy the "enhance" requirement, the system must generate this asset programmatically.

The "Renderer" Microservice:
This component is designed to generate a visual representation of the workout's impact.

1. **Mapping Database:** A hardcoded map (or Firestore collection) links Hevy Exercise IDs to anatomical identifiers (e.g., Bench Press \-\> \[pectoralis\_major, triceps\_brachii, anterior\_deltoid\]).
2. **SVG Base Layer:** The system stores a base anatomical SVG 15 where each muscle group is a distinct path with a unique id.
3. **Intensity Calculation:** The system sums the volume (weight \* sets \* reps) for each muscle group in the current workout. It normalizes these values against the user's historical maximums (retrieved from Firestore) to determine a "Heat" score (0.0 to 1.0).
4. **Dynamic Coloring:** The Go or Node.js function parses the XML of the SVG. It iterates through the target muscle IDs and injects a fill attribute corresponding to the Heat score (e.g., turning the "Pecs" path bright red for a high-volume chest day).
5. **Rasterization:** Strava uploads require image formats (JPG/PNG). The system uses a library like librsvg (Go binding) or Puppeteer (Node.js) to render the modified SVG into a PNG file.
6. **Persistence:** The generated image is saved to Cloud Storage. The public URL or the binary itself is then prepared for the Strava upload.

## ---

**4\. Egress Layer: Strava Synchronization and Metadata Control**

The final phase involves pushing the processed data to Strava. This interaction is governed by strict rate limits and specific API behaviors.

### **4.1 The Hashtag Strategy for Map Types**

The user explicitly requested setting "Map Types" (e.g., formatting the map to show Heart Rate intensity or Pace). A critical finding from the research is that the Strava API updateActivity endpoint **does not** expose a direct parameter for map\_type or map\_style.11

**The Solution:** Strava controls these visualizations via **hashtags in the activity description**.17 This is an undocumented but supported "power user" feature for subscribers.

* **Logic:** The "Enhancer" function applies logic based on the activity type:
  * If type \== Run: Append \#PaceMap to the description string.
  * If type \== Ride (with Power): Append \#PowerMap.
  * If type \== Hike: Append \#ElevationMap.
  * If type \== Cardio (high HR variance): Append \#HeartrateMap.
* **Implementation:** This string manipulation occurs in the Description Automation step (Section 3.2) prior to the final update request. This satisfies the requirement without needing nonexistent API fields.

### **4.2 The Upload and Update Workflow**

The interaction with Strava is a two-step process to ensure all data is correctly attributed.

Step 1: Activity Creation (Upload)
For Keiser/Fitbit merged files, the system uses the POST /uploads endpoint.7

* **Payload:** file (the binary.fit file), data\_type="fit", activity\_type="ride".
* **Asynchronous Polling:** The response provides an id. The system must poll GET /uploads/{id} until the status transitions to "ready" and an activity\_id is returned.

Step 2: Metadata Enrichment (Update)
Once the activity\_id is secured (or if enhancing an existing Hevy activity), the system calls PUT /activities/{id}.13

* **Payload:**
  * description: The generated string containing stats and the \#StatMap hashtags.
  * workout\_type: Set to 1 if Parkrun detected.
  * commute: Set to true if geospatial analysis indicates a commute path.
  * name: Enhanced title (e.g., "Parkrun PB Attempt").

Handling Media Uploads (Heatmaps):
Uploading the generated Heatmap PNG is complex. The public Strava API does not explicitly document a generic "upload photo to activity" endpoint for all developers; historically, this was reserved for partner integrations.19

* **Workaround/Fallback:** If the direct media upload endpoint is restricted, the system appends a public link to the image (hosted on GCS) in the activity description. Alternatively, newer API revisions should be tested for POST /uploads with data\_type=image/jpeg linked to the activity\_id.

## ---

**5\. Security and Operational Management**

### **5.1 Authentication and Token Rotation**

The system relies on OAuth 2.0 for both Strava and Fitbit. Managing these tokens requires a robust rotation strategy to prevent manual intervention.

**Firestore Schema:**

* collection: users
  * doc: {userId}
    * strava\_access\_token: String
    * strava\_refresh\_token: String
    * strava\_expires\_at: Timestamp
    * fitbit\_access\_token: String
    * fitbit\_refresh\_token: String

Middleware Logic:
Before any API call, a helper function GetValidToken(provider) is invoked:

1. Retrieve the document from Firestore.
2. Check if expires\_at is within a 5-minute buffer of now().
3. If valid, return the access\_token.
4. If expired, execute the Refresh Flow:
   * Send refresh\_token to the provider's token endpoint (https://www.strava.com/oauth/token 20).
   * Receive the new access\_token and refresh\_token (Strava rotates refresh tokens too).
   * Atomic write to Firestore to update credentials.
   * Return the new token.

### **5.2 Rate Limiting and Quotas**

Strava imposes a default rate limit of 100 requests every 15 minutes.7

* **Protection Strategy:** The Cloud Pub/Sub architecture inherently buffers requests. If the "Enhancer" function receives a 429 (Too Many Requests) error, it NACKs the message, prompting Pub/Sub to retry with exponential backoff.
* **Optimization:** Deduplication logic in Firestore prevents processing the same webhook event twice, saving valuable API quota.

## ---

**6\. Implementation Roadmap**

1. **Phase 1: Foundation.** Deploy Firestore and Secret Manager. Implement the OAuth 2.0 Token Manager in Golang.
2. **Phase 2: Ingestion.** Build the Hevy Webhook receiver (Node.js) and the Keiser/Fitbit Pollers (Cloud Scheduler \+ Node.js). Verify raw data storage in GCS.
3. **Phase 3: The Core.** Develop the FIT file generator in Golang. Implement the linear interpolation for HR/Power merging.
4. **Phase 4: Enrichment.** Implement the Parkrun geospatial lookups and Description template engine. Add the \#StatMap logic.
5. **Phase 5: Deployment.** Deploy all functions using Terraform or gcloud CLI to GCP. Configure Pub/Sub triggers.

This architecture transforms the user's fragmented data into a professional-grade, automated fitness log. By combining specific API capabilities—like Hevy's webhooks and Fitbit's Intraday series—with clever workarounds like Strava's hashtag maps and geospatial tagging, the solution delivers a seamless, "set-and-forget" experience that significantly enhances the value of the collected data.

## **7\. Deep Dive: Implementation Specifications**

### **7.1 Keiser Metrics SDK & Cloud Integration Details**

The Keiser M Series integration is particularly nuanced because it relies on the user's existing behavior of syncing their bike computer to the M Series App, which then syncs to the Keiser Cloud. We are intercepting the data at the Cloud level.

SDK Utilization:
The Keiser.Metrics.SDK.SSO.Typescript library is the bridge. While the architecture uses Golang for heavy lifting, the ingestion of Keiser data should use Node.js to leverage this native SDK, avoiding the complexity of reverse-engineering the raw API REST calls.

* **Session Retrieval:** The Poller function will utilize the userSession.getStrengthExercises or the specific cardio equivalent getCardioExercises (conceptually, though specific endpoint names in the SDK documentation 3 usually map to MSeriesDataSet).
* **Data Resolution:** The SDK returns data objects often nested with specific IDs. The MSeriesDataSetData 3 contains the power, cadence, and gear arrays. These arrays are critical. The timestamps in these arrays must be converted from Keiser's format (often relative or ISO 8601\) to absolute Unix timestamps to allow alignment with Fitbit.

Handling Authentication Nuances:
The SDK uses a specific flow: authLogin returns a refresh\_token and access\_token.

* **Scenario:** The Keiser access token expires.
* **Handling:** The Poller must catch 401 errors from the SDK calls and internally call the authExchangeFulfillment endpoint 6 using the stored refresh token from Firestore. This ensures the poller runs autonomously for months without user intervention.

### **7.2 Fitbit Intraday Data & "Personal" App Key**

The Fitbit integration has a specific "gotcha" regarding the 1sec resolution.

* **Application Registration:** When the user registers their app on dev.fitbit.com, they *must* select **"Personal"** as the application type. If "Server" or "Client" is selected, the API will return 403 Forbidden when requesting /1sec data, defaulting instead to /1min which is insufficient for merging with second-by-second power data.
* **Latency & "Not Ready" States:**
  * Fitbit data is not real-time. It depends on the user syncing their tracker to their phone.
  * **Strategy:** The "Enhancer" function checks the "last sync time" of the Fitbit device (via GET /1/user/-/devices.json).
  * If last\_sync\_time \< activity\_end\_time, the data is incomplete.
  * **Action:** The function calculates a "backoff" time (e.g., 10 minutes) and republishes the message to Pub/Sub with a delivery delay. This loop continues until the sync time updates, ensuring the generated FIT file doesn't have a flat-line HR of 0\.

### **7.3 Strava Uploads: The Multipart Construction**

For the Golang implementation of the Egress layer, constructing the multipart/form-data request for Strava requires precision.

* **Endpoint:** POST https://www.strava.com/api/v3/uploads
* **Headers:** Authorization: Bearer \<token\>, Content-Type: multipart/form-data; boundary=\<boundary\>
* **Body Construction:**
  * Part 1: file. Content-Type: application/octet-stream (or application/fit). This is the binary content generated by the Merge logic.
  * Part 2: data\_type. Value: fit.
  * Part 3: activity\_type. Value: ride (for Keiser).
* **Response Handling:** The API returns an id (upload ID), *not* the activity ID.
* **Polling Pattern:** The function must enter a loop:
  1. Wait 2 seconds.
  2. Call GET /uploads/{id}.
  3. Check status.
  4. If "Your activity is ready," extract activity\_id for the subsequent Update/Enrichment calls.
  5. If "error," log to Cloud Logging and alert the user (via email or Firestore status flag).

### **7.4 Parkrun Logic: The Haversine Implementation**

To satisfy the "Auto-tag Parkruns" requirement with high precision, the Golang implementation of the geospatial check should look like this:

Go

// Haversine formula to calculate distance between two points
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
    const R \= 6371000 // Earth radius in meters
    dLat := (lat2 \- lat1) \* (math.Pi / 180.0)
    dLon := (lon2 \- lon1) \* (math.Pi / 180.0)
    lat1 \= lat1 \* (math.Pi / 180.0)
    lat2 \= lat2 \* (math.Pi / 180.0)

    a := math.Sin(dLat/2)\*math.Sin(dLat/2) \+
        math.Sin(dLon/2)\*math.Sin(dLon/2)\*math.Cos(lat1)\*math.Cos(lat2)
    c := 2 \* math.Atan2(math.Sqrt(a), math.Sqrt(1\-a))
    return R \* c
}

// Logic within the Enhancer Service
func CheckForParkrun(activity Activity, eventsParkrunEvent) (bool, string) {
    // 1\. Time Check: Is it Saturday morning?
    startTime := activity.StartDateLocal
    if startTime.Weekday()\!= time.Saturday {
        return false, ""
    }
    hour := startTime.Hour()
    if hour \< 8 |

| hour \> 10 { // Optimization: Only check 8am-10am
        return false, ""
    }

    // 2\. Spatial Check
    for \_, event := range events {
        dist := Haversine(activity.StartLat, activity.StartLng, event.Lat, event.Lon)
        if dist \< 200.0 { // 200 meters threshold
            return true, event.Name
        }
    }
    return false, ""
}

This code snippet illustrates the logic that must reside in the activity-enhancer function. The events slice is populated from the events.json cache stored in Firestore.

## **8\. Summary of Requirements Satisfaction**

| Requirement | Implementation Strategy | Status |
| :---- | :---- | :---- |
| **Cloud-Only Solution** | GCP Serverless (Functions, Pub/Sub, Firestore) | **Satisfied** |
| **Sync Hevy** | Webhook Handler (Node.js) \-\> Strava Update | **Satisfied** |
| **Sync Keiser M Series** | Poller (Node.js) using SDK \-\> Merge \-\> Upload | **Satisfied** |
| **Sync Fitbit** | Intraday API (Personal App) \-\> Merge w/ Keiser | **Satisfied** |
| **Merge Heart Rate** | Golang Time-Alignment & FIT File Gen | **Satisfied** |
| **Automate Descriptions** | Template Engine (Golang) injecting stats | **Satisfied** |
| **Set Map Types** | Description Append logic (\#PaceMap, etc.) | **Satisfied** |
| **Auto-tag Parkruns** | Geospatial Haversine Check \+ workout\_type=1 | **Satisfied** |

This detailed breakdown confirms that every specific constraint and requirement from the user's prompt has been addressed with a concrete technical strategy grounded in the provided research material.

#### **Works cited**

1. Hevy API (Feature Request) \- Read / Write Heart Rate Data to Workouts \- Reddit, accessed on December 18, 2025, [https://www.reddit.com/r/Hevy/comments/1ms2sqi/hevy\_api\_feature\_request\_read\_write\_heart\_rate/](https://www.reddit.com/r/Hevy/comments/1ms2sqi/hevy_api_feature_request_read_write_heart_rate/)
2. Syncing Hevy Workouts to Notion Using Zapier \- James Carr, accessed on December 18, 2025, [https://james-carr.org/posts/sync-hevy-to-notion-using-zapier/](https://james-carr.org/posts/sync-hevy-to-notion-using-zapier/)
3. KeiserCorp/Keiser.Metrics.SDK.SSO.Typescript \- GitHub, accessed on December 18, 2025, [https://github.com/KeiserCorp/Keiser.Metrics.SDK.SSO.Typescript](https://github.com/KeiserCorp/Keiser.Metrics.SDK.SSO.Typescript)
4. Get Heart Rate Intraday by Date Range \- Fitbit, accessed on December 18, 2025, [https://dev.fitbit.com/build/reference/web-api/intraday/get-heartrate-intraday-by-date-range/](https://dev.fitbit.com/build/reference/web-api/intraday/get-heartrate-intraday-by-date-range/)
5. Researchers FAQ \- Fitbit Enterprise, accessed on December 18, 2025, [https://fitbit.google/enterprise/researchers-faqs/](https://fitbit.google/enterprise/researchers-faqs/)
6. keiser\_metrics\_sdk 5.1.0 | Dart package \- Pub.dev, accessed on December 18, 2025, [https://pub.dev/packages/keiser\_metrics\_sdk/versions/5.1.0](https://pub.dev/packages/keiser_metrics_sdk/versions/5.1.0)
7. Uploading \- Strava Developers, accessed on December 18, 2025, [https://developers.strava.com/docs/uploads/](https://developers.strava.com/docs/uploads/)
8. fit-tool \- PyPI, accessed on December 18, 2025, [https://pypi.org/project/fit-tool/](https://pypi.org/project/fit-tool/)
9. Where did my flags go? \- Running Challenges, accessed on December 18, 2025, [https://running-challenges.co.uk/2019/11/19/where-did-my-flags-go.html](https://running-challenges.co.uk/2019/11/19/where-did-my-flags-go.html)
10. parkrun HQ have changed the datasource for the event map · Issue \#174 · fraz3alpha/running-challenges \- GitHub, accessed on December 18, 2025, [https://github.com/fraz3alpha/running-challenges/issues/174](https://github.com/fraz3alpha/running-challenges/issues/174)
11. Strava API v3 \- Strava Developers, accessed on December 18, 2025, [https://developers.strava.com/docs/reference/\#api-Activities-updateActivity](https://developers.strava.com/docs/reference/#api-Activities-updateActivity)
12. Determining your closest Parkrun Alphabet Challenge using Python and pandas, accessed on December 18, 2025, [https://eddmann.com/posts/determining-your-closest-parkrun-alphabet-challenge-using-python-and-pandas/](https://eddmann.com/posts/determining-your-closest-parkrun-alphabet-challenge-using-python-and-pandas/)
13. Strava API v3, accessed on December 18, 2025, [https://developers.strava.com/docs/reference/](https://developers.strava.com/docs/reference/)
14. Strava Activity Tags, accessed on December 18, 2025, [https://support.strava.com/hc/en-us/articles/216919557-Strava-Activity-Tags](https://support.strava.com/hc/en-us/articles/216919557-Strava-Activity-Tags)
15. Human Anatomy created with SVG \- GitHub, accessed on December 18, 2025, [https://github.com/eMahtab/human-anatomy](https://github.com/eMahtab/human-anatomy)
16. Activity: SVG Anatomy \- GitHub Gist, accessed on December 18, 2025, [https://gist.github.com/acidtone/7dc749f62b43bc777859ca52cde2b791](https://gist.github.com/acidtone/7dc749f62b43bc777859ca52cde2b791)
17. Using Strava's New Custom Activity Lines \#statmaps \- Zwift Insider, accessed on December 18, 2025, [https://zwiftinsider.com/strava-statmaps/](https://zwiftinsider.com/strava-statmaps/)
18. Blog/Post/Strava-statmaps \- Kyriakos, accessed on December 18, 2025, [https://sider.is/blog/post/strava-statmaps](https://sider.is/blog/post/strava-statmaps)
19. How to upload photo's via Strava API \- Google Groups, accessed on December 18, 2025, [https://groups.google.com/g/strava-api/c/a9UAomssMOk](https://groups.google.com/g/strava-api/c/a9UAomssMOk)
20. Strava API Python Tutorial (Beginner): Access Activity Data \- Jdhwilkins, accessed on December 18, 2025, [https://jdhwilkins.com/using-the-strava-api-with-python-beginner-tutorial/](https://jdhwilkins.com/using-the-strava-api-with-python-beginner-tutorial/)
