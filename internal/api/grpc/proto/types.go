// Package proto contains manual Go types mirroring the protobuf definitions
// in proto/skillbox/v1/skillbox.proto. These will be replaced by generated
// code when buf/protoc tooling is set up.
package proto

// RunSkillRequest mirrors the RunSkillRequest protobuf message.
type RunSkillRequest struct {
	Skill   string
	Version string
	Input   map[string]interface{}
	Env     map[string]string
}

// RunSkillResponse mirrors the RunSkillResponse protobuf message.
type RunSkillResponse struct {
	ExecutionId string
	Status      string
	Output      map[string]interface{}
	FilesUrl    string
	FilesList   []string
	Logs        string
	DurationMs  int64
	Error       string
}

// GetExecutionRequest mirrors the GetExecutionRequest protobuf message.
type GetExecutionRequest struct {
	Id string
}

// GetExecutionResponse mirrors the GetExecutionResponse protobuf message.
type GetExecutionResponse struct {
	ExecutionId string
	Status      string
	Output      map[string]interface{}
	FilesUrl    string
	FilesList   []string
	Logs        string
	DurationMs  int64
	Error       string
}

// GetExecutionLogsRequest mirrors the GetExecutionLogsRequest protobuf message.
type GetExecutionLogsRequest struct {
	Id string
}

// GetExecutionLogsResponse mirrors the GetExecutionLogsResponse protobuf message.
type GetExecutionLogsResponse struct {
	Logs string
}

// ListSkillsRequest mirrors the ListSkillsRequest protobuf message.
type ListSkillsRequest struct{}

// ListSkillsResponse mirrors the ListSkillsResponse protobuf message.
type ListSkillsResponse struct {
	Skills []*SkillInfo
}

// SkillInfo mirrors the SkillInfo protobuf message.
type SkillInfo struct {
	Name        string
	Version     string
	Description string
}

// GetSkillRequest mirrors the GetSkillRequest protobuf message.
type GetSkillRequest struct {
	Name    string
	Version string
}

// GetSkillResponse mirrors the GetSkillResponse protobuf message.
type GetSkillResponse struct {
	Name        string
	Version     string
	Description string
	Lang        string
	Content     string
}

// DeleteSkillRequest mirrors the DeleteSkillRequest protobuf message.
type DeleteSkillRequest struct {
	Name    string
	Version string
}

// DeleteSkillResponse mirrors the DeleteSkillResponse protobuf message.
type DeleteSkillResponse struct{}

// HealthCheckRequest mirrors the HealthCheckRequest protobuf message.
type HealthCheckRequest struct{}

// HealthCheckResponse mirrors the HealthCheckResponse protobuf message.
type HealthCheckResponse struct {
	Status string
}
