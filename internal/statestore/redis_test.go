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

package statestore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/rs/xid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/internal/config"
	"open-match.dev/open-match/internal/pb"
)

func TestStatestoreSetup(t *testing.T) {
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()
}

func TestTicketLifecycle(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	// Initialize test data
	id := xid.New().String()
	ticket := &pb.Ticket{
		Id: id,
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"testindex1": {Kind: &structpb.Value_NumberValue{NumberValue: 42}},
			},
		},
		Assignment: &pb.Assignment{
			Connection: "test-tbd",
		},
	}

	// Validate that GetTicket fails for a Ticket that does not exist.
	_, err := service.GetTicket(context.Background(), id)
	assert.NotNil(err)
	assert.Equal(status.Code(err), codes.NotFound)

	// Validate nonexisting Ticket deletion
	err = service.DeleteTicket(context.Background(), id)
	assert.Nil(err)

	// Validate nonexisting Ticket deindexing
	err = service.DeindexTicket(context.Background(), id)
	assert.Nil(err)

	// Validate Ticket creation
	err = service.CreateTicket(context.Background(), ticket)
	assert.Nil(err)

	// Validate Ticket retrival
	result, err := service.GetTicket(context.Background(), ticket.Id)
	assert.Nil(err)
	assert.NotNil(result)
	assert.Equal(ticket.Id, result.Id)
	assert.Equal(ticket.Properties.Fields["testindex1"].GetNumberValue(), result.Properties.Fields["testindex1"].GetNumberValue())
	assert.Equal(ticket.Assignment.Connection, result.Assignment.Connection)

	// Validate Ticket deletion
	err = service.DeleteTicket(context.Background(), id)
	assert.Nil(err)

	_, err = service.GetTicket(context.Background(), id)
	assert.NotNil(err)
}

func TestTicketIndexing(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("ticket.no.%d", i)

		ticket := &pb.Ticket{
			Id: id,
			Properties: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"testindex1": {Kind: &structpb.Value_NumberValue{NumberValue: float64(i)}},
					"testindex2": {Kind: &structpb.Value_NumberValue{NumberValue: 0.5}},
				},
			},
			Assignment: &pb.Assignment{
				Connection: "test-tbd",
			},
		}

		err := service.CreateTicket(context.Background(), ticket)
		assert.Nil(err)

		err = service.IndexTicket(context.Background(), ticket)
		assert.Nil(err)
	}

	// Remove one ticket, to test that it doesn't fall over.
	err := service.DeleteTicket(context.Background(), "ticket.no.5")
	assert.Nil(err)

	// Remove ticket from index, should not show up.
	err = service.DeindexTicket(context.Background(), "ticket.no.6")
	assert.Nil(err)

	found := make(map[string]struct{})

	filters := []*pb.Filter{
		{
			Attribute: "testindex1",
			Min:       2.5,
			Max:       8.5,
		},
		{
			Attribute: "testindex2",
			Min:       0.49,
			Max:       0.51,
		},
	}

	err = service.FilterTickets(context.Background(), filters, 2, func(tickets []*pb.Ticket) error {
		assert.True(len(tickets) <= 2)
		for _, ticket := range tickets {
			found[ticket.Id] = struct{}{}
		}
		return nil
	})
	assert.Nil(err)

	assert.Equal(len(found), 4)
	assert.Contains(found, "ticket.no.3")
	assert.Contains(found, "ticket.no.4")
	assert.Contains(found, "ticket.no.7")
	assert.Contains(found, "ticket.no.8")
}

func TestGetAssignmentBeforeSet(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	var assignmentResp *pb.Assignment

	err := service.GetAssignments(context.Background(), "id", func(assignment *pb.Assignment) error {
		assignmentResp = assignment
		return nil
	})
	// GetAssignment failed because the ticket does not exists
	assert.Equal(status.Convert(err).Code(), codes.NotFound)
	assert.Nil(assignmentResp)
}

func TestUpdateAssignmentFatal(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	var assignmentResp *pb.Assignment

	err := service.UpdateAssignments(context.Background(), []string{"id"}, &pb.Assignment{})
	// UpdateAssignment failed because the ticket does not exists
	assert.Equal(status.Convert(err).Code(), codes.NotFound)
	assert.Nil(assignmentResp)

	// Now create a ticket and the state store service
	err = service.CreateTicket(context.Background(), &pb.Ticket{
		Id:         "1",
		Assignment: &pb.Assignment{Connection: "2"},
	})
	assert.Nil(err)

	// Try to update the assignmets with the ticket created and some non-existed tickets
	err = service.UpdateAssignments(context.Background(), []string{"1", "2", "3"}, &pb.Assignment{})
	// UpdateAssignment failed because the ticket does not exists
	assert.Equal(status.Convert(err).Code(), codes.NotFound)
	assert.Nil(assignmentResp)

	// Verify the transaction behavior of the UpdateAssignment.
	ticket, err := service.GetTicket(context.Background(), "1")
	assert.Equal(&pb.Assignment{}, ticket.Assignment)
	assert.Nil(err)
}

func TestGetAssignmentNormal(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	err := service.CreateTicket(context.Background(), &pb.Ticket{
		Id:         "1",
		Assignment: &pb.Assignment{Connection: "2"},
	})
	assert.Nil(err)

	var assignmentResp *pb.Assignment
	ctx, cancel := context.WithCancel(context.Background())
	callbackCount := 0
	returnedErr := errors.New("some errors")

	err = service.GetAssignments(ctx, "1", func(assignment *pb.Assignment) error {
		assignmentResp = assignment

		if callbackCount == 5 {
			cancel()
			return returnedErr
		} else if callbackCount > 0 {
			// Test the assignment returned was successfully passed in to the callback function
			assert.Equal(assignmentResp.Connection, "2")
		}

		callbackCount++
		return nil
	})

	// Test GetAssignments was retried for 5 times and returned with expected error
	assert.Equal(5, callbackCount)
	assert.Equal(returnedErr, err)
}

func TestUpdateAssignmentNormal(t *testing.T) {
	// Create State Store
	assert := assert.New(t)
	cfg := createRedis(t)
	service := New(cfg)
	assert.NotNil(service)
	defer service.Close()

	// Create a ticket without assignment
	err := service.CreateTicket(context.Background(), &pb.Ticket{
		Id: "1",
	})
	assert.Nil(err)
	// Create a ticket already with an assignment
	err = service.CreateTicket(context.Background(), &pb.Ticket{
		Id:         "3",
		Assignment: &pb.Assignment{Connection: "4"},
	})
	assert.Nil(err)

	fakeAssignment := &pb.Assignment{Connection: "Halo"}
	err = service.UpdateAssignments(context.Background(), []string{"1", "3"}, fakeAssignment)
	assert.Nil(err)
	// Verify the transaction behavior of the UpdateAssignment.
	ticket, err := service.GetTicket(context.Background(), "1")
	assert.Equal(fakeAssignment.Connection, ticket.Assignment.Connection)
	assert.Nil(err)
	// Verify the transaction behavior of the UpdateAssignment.
	ticket, err = service.GetTicket(context.Background(), "3")
	assert.Equal(fakeAssignment.Connection, ticket.Assignment.Connection)
	assert.Nil(err)

}

func createRedis(t *testing.T) config.View {
	cfg := viper.New()
	mredis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("cannot create redis %s", err)
	}

	cfg.Set("redis.hostname", mredis.Host())
	cfg.Set("redis.port", mredis.Port())
	cfg.Set("redis.pool.maxIdle", 1000)
	cfg.Set("redis.pool.idleTimeout", time.Second)
	cfg.Set("redis.pool.healthCheckTimeout", 100*time.Millisecond)
	cfg.Set("redis.pool.maxActive", 1000)
	cfg.Set("redis.expiration", 42000)
	cfg.Set("backoff.initialInterval", 100*time.Millisecond)
	cfg.Set("backoff.randFactor", 0.5)
	cfg.Set("backoff.multiplier", 0.5)
	cfg.Set("backoff.maxInterval", 300*time.Millisecond)
	cfg.Set("backoff.maxElapsedTime", 100*time.Millisecond)
	cfg.Set("playerIndices", []string{"testindex1", "testindex2"})

	return cfg
}
