// Package grpc implements the Skillbox gRPC server. It exposes the same
// functionality as the REST API (execution, skill management, health checks)
// over gRPC, using the runner, store, and registry packages.
package grpc

import (
	"context"
	"encoding/json"
	"net"
	"os"

	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/devs-group/skillbox/internal/api/grpc/proto"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/runner"
	"github.com/devs-group/skillbox/internal/store"
)

// Server is the Skillbox gRPC server. It holds references to the core
// dependencies (runner, store, registry) and exposes them through the
// ExecutionService, SkillService, and HealthService RPCs.
type Server struct {
	runner   *runner.Runner
	store    *store.Store
	registry *registry.Registry
	grpc     *grpclib.Server
}

// NewServer creates a new gRPC server with the given dependencies. The
// underlying grpc.Server is created and service handlers are registered.
// Call Serve() to start accepting connections.
func NewServer(r *runner.Runner, s *store.Store, reg *registry.Registry) *Server {
	srv := grpclib.NewServer()
	server := &Server{
		runner:   r,
		store:    s,
		registry: reg,
		grpc:     srv,
	}
	// Service registration will be added here when generated protobuf
	// code is available (e.g. pb.RegisterExecutionServiceServer(srv, server)).
	return server
}

// Serve starts the gRPC server on the given listener. It blocks until
// the server is stopped or an error occurs.
func (s *Server) Serve(lis net.Listener) error {
	return s.grpc.Serve(lis)
}

// Stop gracefully shuts down the gRPC server, waiting for in-flight
// RPCs to complete.
func (s *Server) Stop() {
	s.grpc.GracefulStop()
}

// GRPCServer returns the underlying grpc.Server instance, which can be
// used for additional configuration such as registering reflection or
// custom interceptors.
func (s *Server) GRPCServer() *grpclib.Server {
	return s.grpc
}

// ---------------------------------------------------------------------------
// ExecutionService implementation
// ---------------------------------------------------------------------------

// RunSkill executes a skill and returns the result. It translates the
// gRPC request into a runner.RunRequest and converts the RunResult back
// to the protobuf response type.
func (s *Server) RunSkill(ctx context.Context, req *pb.RunSkillRequest) (*pb.RunSkillResponse, error) {
	if req.Skill == "" {
		return nil, status.Error(codes.InvalidArgument, "'skill' is required")
	}

	version := req.Version
	if version == "" {
		version = "latest"
	}

	// Convert the input map to JSON for the runner.
	var inputJSON json.RawMessage
	if req.Input != nil {
		data, err := json.Marshal(req.Input)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid input: %v", err)
		}
		inputJSON = data
	}

	result, err := s.runner.Run(ctx, runner.RunRequest{
		Skill:   req.Skill,
		Version: version,
		Input:   inputJSON,
		Env:     req.Env,
		// TenantID would come from gRPC metadata in a full implementation.
	})
	if err != nil {
		return nil, runnerErrorToGRPC(err)
	}

	return runResultToResponse(result), nil
}

// GetExecution retrieves an execution record by ID.
func (s *Server) GetExecution(ctx context.Context, req *pb.GetExecutionRequest) (*pb.GetExecutionResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "execution id is required")
	}

	exec, err := s.store.GetExecution(ctx, req.Id)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Error(codes.NotFound, "execution not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve execution: %v", err)
	}

	return executionToResponse(exec), nil
}

// GetExecutionLogs retrieves just the logs for an execution.
func (s *Server) GetExecutionLogs(ctx context.Context, req *pb.GetExecutionLogsRequest) (*pb.GetExecutionLogsResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "execution id is required")
	}

	exec, err := s.store.GetExecution(ctx, req.Id)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Error(codes.NotFound, "execution not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve execution: %v", err)
	}

	return &pb.GetExecutionLogsResponse{Logs: exec.Logs}, nil
}

// ---------------------------------------------------------------------------
// SkillService implementation
// ---------------------------------------------------------------------------

// ListSkills returns metadata for all skills in the registry for the
// current tenant. It reads from the PostgreSQL metadata cache which
// includes descriptions, falling back to MinIO listing if needed.
func (s *Server) ListSkills(ctx context.Context, req *pb.ListSkillsRequest) (*pb.ListSkillsResponse, error) {
	// TenantID would come from gRPC metadata in a full implementation.
	tenantID := ""

	// Try the database first — it has descriptions.
	records, err := s.store.ListSkills(ctx, tenantID)
	if err == nil && len(records) > 0 {
		resp := &pb.ListSkillsResponse{
			Skills: make([]*pb.SkillInfo, 0, len(records)),
		}
		for _, rec := range records {
			resp.Skills = append(resp.Skills, &pb.SkillInfo{
				Name:        rec.Name,
				Version:     rec.Version,
				Description: rec.Description,
			})
		}
		return resp, nil
	}

	// Fall back to registry listing.
	skills, err := s.registry.List(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list skills: %v", err)
	}

	resp := &pb.ListSkillsResponse{
		Skills: make([]*pb.SkillInfo, 0, len(skills)),
	}
	for _, sk := range skills {
		resp.Skills = append(resp.Skills, &pb.SkillInfo{
			Name:    sk.Name,
			Version: sk.Version,
		})
	}

	return resp, nil
}

// GetSkill retrieves the full metadata for a specific skill version,
// including the SKILL.md instructions body.
func (s *Server) GetSkill(ctx context.Context, req *pb.GetSkillRequest) (*pb.GetSkillResponse, error) {
	if req.Name == "" || req.Version == "" {
		return nil, status.Error(codes.InvalidArgument, "skill name and version are required")
	}

	// TenantID would come from gRPC metadata in a full implementation.
	tenantID := ""

	loaded, err := registry.LoadSkill(ctx, s.registry, tenantID, req.Name, req.Version)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve skill: %v", err)
	}
	defer func() {
		if loaded.Dir != "" {
			os.RemoveAll(loaded.Dir)
		}
	}()

	sk := loaded.Skill
	var timeout string
	if sk.Timeout > 0 {
		timeout = sk.Timeout.String()
	}

	return &pb.GetSkillResponse{
		Name:         sk.Name,
		Version:      sk.Version,
		Description:  sk.Description,
		Lang:         sk.Lang,
		Content:      sk.Instructions,
		Image:        sk.Image,
		Timeout:      timeout,
		Instructions: sk.Instructions,
	}, nil
}

// DeleteSkill removes a skill version from the registry.
func (s *Server) DeleteSkill(ctx context.Context, req *pb.DeleteSkillRequest) (*pb.DeleteSkillResponse, error) {
	if req.Name == "" || req.Version == "" {
		return nil, status.Error(codes.InvalidArgument, "skill name and version are required")
	}

	// TenantID would come from gRPC metadata in a full implementation.
	tenantID := ""
	if err := s.registry.Delete(ctx, tenantID, req.Name, req.Version); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete skill: %v", err)
	}

	return &pb.DeleteSkillResponse{}, nil
}

// ---------------------------------------------------------------------------
// HealthService implementation
// ---------------------------------------------------------------------------

// Check returns the health status of the server.
func (s *Server) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Status: "ok"}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runnerErrorToGRPC maps runner package errors to appropriate gRPC status codes.
func runnerErrorToGRPC(err error) error {
	switch {
	case err == runner.ErrSkillNotFound:
		return status.Error(codes.NotFound, "skill not found")
	case err == runner.ErrImageNotAllowed:
		return status.Error(codes.FailedPrecondition, "skill image is not in the allowlist")
	case err == runner.ErrTimeout:
		return status.Error(codes.DeadlineExceeded, "execution timed out")
	default:
		return status.Errorf(codes.Internal, "execution failed: %v", err)
	}
}

// runResultToResponse converts a runner.RunResult to a gRPC RunSkillResponse.
func runResultToResponse(r *runner.RunResult) *pb.RunSkillResponse {
	resp := &pb.RunSkillResponse{
		ExecutionId: r.ExecutionID,
		Status:      r.Status,
		FilesUrl:    r.FilesURL,
		FilesList:   r.FilesList,
		Logs:        r.Logs,
		DurationMs:  r.DurationMs,
	}

	// Convert output JSON to map.
	if r.Output != nil {
		var m map[string]interface{}
		if err := json.Unmarshal(r.Output, &m); err == nil {
			resp.Output = m
		}
	}

	if r.Error != nil {
		resp.Error = *r.Error
	}

	return resp
}

// executionToResponse converts a store.Execution to a gRPC GetExecutionResponse.
func executionToResponse(e *store.Execution) *pb.GetExecutionResponse {
	resp := &pb.GetExecutionResponse{
		ExecutionId: e.ID,
		Status:      e.Status,
		FilesUrl:    e.FilesURL,
		FilesList:   e.FilesList,
		Logs:        e.Logs,
		DurationMs:  e.DurationMs,
	}

	// Convert output JSON to map.
	if e.Output != nil {
		var m map[string]interface{}
		if err := json.Unmarshal(e.Output, &m); err == nil {
			resp.Output = m
		}
	}

	if e.Error != nil {
		resp.Error = *e.Error
	}

	return resp
}

