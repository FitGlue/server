package e2e

import (
	"fmt"

	"github.com/cucumber/godog"
)

// apiTestState holds any state that needs to be shared between steps in a scenario.
type apiTestState struct {
	provider     string
	distance     string
	responseCode int
}

func (s *apiTestState) theLocalDockercomposeEnvironmentIsRunning() error {
	// e.g., setup HTTP clients to ping gateways
	return godog.ErrPending
}

func (s *apiTestState) aTestUserExistsViaTheMockAPI() error {
	return godog.ErrPending
}

func (s *apiTestState) aMockWebhookPayloadFromSimulatingARunIsPostedToTheApiwebhook(provider, distance string) error {
	s.provider = provider
	s.distance = distance
	fmt.Printf("[DEBUG] Preparing to post %s payload for a %s run to api-webhook...\n", provider, distance)
	return godog.ErrPending
}

func (s *apiTestState) theRequestShouldPassOpenAPIValidationAndReturn(expectedCode int) error {
	fmt.Printf("[DEBUG] Asserting that OpenAPI validation passes and returns HTTP %d...\n", expectedCode)
	s.responseCode = expectedCode
	return godog.ErrPending
}

func (s *apiTestState) thePipelineShouldProcessAndPublishToPubSub() error {
	return godog.ErrPending
}

func (s *apiTestState) theMockDestinationShouldRegisterASuccessfulUpload() error {
	return godog.ErrPending
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	// Create a new fresh state for each scenario execution
	state := &apiTestState{}

	// Exact string matches
	ctx.Step(`^the local docker-compose environment is running$`, state.theLocalDockercomposeEnvironmentIsRunning)
	ctx.Step(`^a test user exists via the mock API$`, state.aTestUserExistsViaTheMockAPI)
	ctx.Step(`^the pipeline should process and publish to Pub/Sub$`, state.thePipelineShouldProcessAndPublishToPubSub)
	ctx.Step(`^the mock destination should register a successful upload$`, state.theMockDestinationShouldRegisterASuccessfulUpload)

	// Parameterized regex matches
	ctx.Step(`^a mock webhook payload from "([^"]*)" simulating a (.+) run is posted to the api-webhook$`, state.aMockWebhookPayloadFromSimulatingARunIsPostedToTheApiwebhook)
	ctx.Step(`^the request should pass OpenAPI validation and return (\d+)$`, state.theRequestShouldPassOpenAPIValidationAndReturn)
}
