// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package testing provides testing primitives for the codebase.
package testing

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/internal/pb"
)

// FakeFrontend is an empty gRPC handler.
type FakeFrontend struct {
}

// CreateTicket will create a new ticket, assign a Ticket id to it and put the
// Ticket in state storage. It will then look through the 'properties' field
// for the attributes defined as indices the matchmakaking config. If the
// attributes exist and are valid integers, they will be indexed. Creating a
// ticket adds the Ticket to the pool of Tickets considered for matchmaking.
func (s *FakeFrontend) CreateTicket(ctx context.Context, req *pb.CreateTicketRequest) (*pb.CreateTicketResponse, error) {
	return &pb.CreateTicketResponse{}, nil
}

// DeleteTicket removes the Ticket from state storage and from corresponding
// configured indices. Deleting the ticket stops the ticket from being
// considered for future matchmaking requests.
func (s *FakeFrontend) DeleteTicket(ctx context.Context, req *pb.DeleteTicketRequest) (*pb.DeleteTicketResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// GetTicket fetches the ticket associated with the specified Ticket id.
func (s *FakeFrontend) GetTicket(ctx context.Context, req *pb.GetTicketRequest) (*pb.Ticket, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// GetAssignments streams matchmaking results from Open Match for the
// provided Ticket id.
func (s *FakeFrontend) GetAssignments(req *pb.GetAssignmentsRequest, stream pb.Frontend_GetAssignmentsServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}
