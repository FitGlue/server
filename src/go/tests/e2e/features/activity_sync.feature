Feature: Activity Synchronization
  As a FitGlue user
  I want my activities to sync across services
  So that they appear in my destination apps

  Scenario Outline: End-to-End Activity Ingestion Pipeline
    Given the local docker-compose environment is running
    And a test user exists via the mock API
    When a mock webhook payload from "<provider>" simulating a <distance> run is posted to the api-webhook
    Then the request should pass OpenAPI validation and return <status_code>
    And the pipeline should process and publish to Pub/Sub
    And the mock destination should register a successful upload

    Examples:
      | provider | distance | status_code |
      | strava   | 5k       | 202         |
      | fitbit   | 10k      | 202         |
      | garmin   | half     | 202         |
