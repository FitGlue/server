package shared

type ActivitySource string

const (
	SourceHevy   ActivitySource = "hevy"
	SourceKeiser ActivitySource = "keiser"
	SourceTest   ActivitySource = "test"
)

type ActivityPayload struct {
	Source          ActivitySource    `json:"source"`
	UserId          string            `json:"userId"`
	Timestamp       string            `json:"timestamp"` // ISO 8601
	OriginalPayload interface{}       `json:"originalPayload"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}
