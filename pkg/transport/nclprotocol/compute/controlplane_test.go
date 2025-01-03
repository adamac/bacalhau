//go:build unit || !integration

package compute_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/bacalhau-project/bacalhau/pkg/lib/envelope"
	"github.com/bacalhau-project/bacalhau/pkg/lib/ncl"
	"github.com/bacalhau-project/bacalhau/pkg/models/messages"
	"github.com/bacalhau-project/bacalhau/pkg/transport/nclprotocol"
	nclprotocolcompute "github.com/bacalhau-project/bacalhau/pkg/transport/nclprotocol/compute"
	ncltest "github.com/bacalhau-project/bacalhau/pkg/transport/nclprotocol/test"
)

type ControlPlaneTestSuite struct {
	suite.Suite
	ctrl             *gomock.Controller
	ctx              context.Context
	cancel           context.CancelFunc
	clock            clock.Clock
	requester        *ncl.MockPublisher
	nodeInfoProvider *ncltest.MockNodeInfoProvider
	healthTracker    *nclprotocolcompute.HealthTracker
	checkpointer     *ncltest.MockCheckpointer
	seqTracker       *nclprotocol.SequenceTracker
	config           nclprotocolcompute.Config
}

func TestControlPlaneTestSuite(t *testing.T) {
	suite.Run(t, new(ControlPlaneTestSuite))
}

func (s *ControlPlaneTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.clock = clock.New() // tickers didn't work properly with mock clock

	// Create mocks
	s.requester = ncl.NewMockPublisher(s.ctrl)
	s.nodeInfoProvider = ncltest.NewMockNodeInfoProvider()
	s.checkpointer = ncltest.NewMockCheckpointer()

	// Create real components
	s.healthTracker = nclprotocolcompute.NewHealthTracker(s.clock)
	s.seqTracker = nclprotocol.NewSequenceTracker()

	// Setup basic config with short intervals for testing
	s.config = nclprotocolcompute.Config{
		NodeID:                 "test-node",
		NodeInfoProvider:       s.nodeInfoProvider,
		Checkpointer:           s.checkpointer,
		HeartbeatInterval:      50 * time.Millisecond,
		NodeInfoUpdateInterval: 100 * time.Millisecond,
		CheckpointInterval:     150 * time.Millisecond,
		RequestTimeout:         50 * time.Millisecond,
		Clock:                  s.clock,
	}
}

func (s *ControlPlaneTestSuite) createControlPlane(
	heartbeatInterval time.Duration,
	nodeInfoInterval time.Duration,
	checkpointInterval time.Duration,
) *nclprotocolcompute.ControlPlane {
	config := nclprotocolcompute.Config{
		NodeID:                 "test-node",
		NodeInfoProvider:       s.nodeInfoProvider,
		Checkpointer:           s.checkpointer,
		HeartbeatInterval:      heartbeatInterval,
		NodeInfoUpdateInterval: nodeInfoInterval,
		CheckpointInterval:     checkpointInterval,
		RequestTimeout:         50 * time.Millisecond,
		Clock:                  s.clock,
	}

	cp, err := nclprotocolcompute.NewControlPlane(nclprotocolcompute.ControlPlaneParams{
		Config:             config,
		Requester:          s.requester,
		HealthTracker:      s.healthTracker,
		IncomingSeqTracker: s.seqTracker,
		CheckpointName:     "test-checkpoint",
	})
	s.Require().NoError(err)
	return cp
}

func (s *ControlPlaneTestSuite) TearDownTest() {
	s.cancel()
	s.ctrl.Finish()
}

func (s *ControlPlaneTestSuite) TestLifecycle() {
	controlPlane := s.createControlPlane(
		50*time.Millisecond,
		100*time.Millisecond,
		150*time.Millisecond)
	defer s.Require().NoError(controlPlane.Stop(s.ctx))

	testCases := []struct {
		name        string
		operation   func() error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "first start succeeds",
			operation:   func() error { return controlPlane.Start(s.ctx) },
			expectError: false,
		},
		{
			name:        "second start fails",
			operation:   func() error { return controlPlane.Start(s.ctx) },
			expectError: true,
			errorMsg:    "already running",
		},
		{
			name:        "first stop succeeds",
			operation:   func() error { return controlPlane.Stop(s.ctx) },
			expectError: false,
		},
		{
			name:        "second stop is noop",
			operation:   func() error { return controlPlane.Stop(s.ctx) },
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := tc.operation()
			if tc.expectError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errorMsg)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ControlPlaneTestSuite) TestHeartbeat() {
	// Create control plane with only heartbeat enabled
	controlPlane := s.createControlPlane(
		50*time.Millisecond, // heartbeat
		1*time.Hour,         // node info - disabled
		1*time.Hour,         // checkpoint - disabled
	)
	defer s.Require().NoError(controlPlane.Stop(s.ctx))

	nodeInfo := s.nodeInfoProvider.GetNodeInfo(s.ctx)
	heartbeatMsg := envelope.NewMessage(messages.HeartbeatRequest{
		NodeID:                 nodeInfo.NodeID,
		AvailableCapacity:      nodeInfo.ComputeNodeInfo.AvailableCapacity,
		QueueUsedCapacity:      nodeInfo.ComputeNodeInfo.QueueUsedCapacity,
		LastOrchestratorSeqNum: s.seqTracker.GetLastSeqNum(),
	}).WithMetadataValue(envelope.KeyMessageType, messages.HeartbeatRequestMessageType)

	s.requester.EXPECT().
		Request(gomock.Any(), ncl.NewPublishRequest(heartbeatMsg)).
		Return(envelope.NewMessage(messages.HeartbeatResponse{}), nil).
		Times(1)

	s.Require().Zero(s.healthTracker.GetHealth().LastSuccessfulHeartbeat)

	s.Require().NoError(controlPlane.Start(s.ctx))
	time.Sleep(50 * time.Millisecond)

	s.Require().Eventually(func() bool {
		health := s.healthTracker.GetHealth()
		return !health.LastSuccessfulHeartbeat.IsZero()
	}, 100*time.Millisecond, 10*time.Millisecond, "Heartbeat did not succeed")
}

func (s *ControlPlaneTestSuite) TestNodeInfoUpdate() {
	// Create control plane with only checkpointing enabled
	controlPlane := s.createControlPlane(
		1*time.Hour,         // heartbeat - disabled
		50*time.Millisecond, // node info
		1*time.Hour,         // checkpoint - disabled
	)
	defer s.Require().NoError(controlPlane.Stop(s.ctx))

	// Start control plane
	s.Require().NoError(controlPlane.Start(s.ctx))

	// update node info after start
	oldInfo := s.nodeInfoProvider.GetNodeInfo(s.ctx)
	newInfo := *oldInfo.Copy()
	newInfo.Labels["new"] = "value"
	s.nodeInfoProvider.SetNodeInfo(newInfo)

	// expect a node info update
	updateMsg := envelope.NewMessage(messages.UpdateNodeInfoRequest{
		NodeInfo: newInfo,
	}).WithMetadataValue(envelope.KeyMessageType, messages.NodeInfoUpdateRequestMessageType)

	s.requester.EXPECT().
		Request(gomock.Any(), ncl.NewPublishRequest(updateMsg)).
		Return(envelope.NewMessage(messages.UpdateNodeInfoResponse{}), nil).
		Times(1)

	// Advance clock to trigger update
	time.Sleep(s.config.NodeInfoUpdateInterval)
	time.Sleep(50 * time.Millisecond) // Allow goroutine to process

	// Verify health tracker state
	health := s.healthTracker.GetHealth()
	s.Require().NotZero(health.LastSuccessfulUpdate)

	// Verify no more updates are sent
	time.Sleep(s.config.NodeInfoUpdateInterval)
	time.Sleep(50 * time.Millisecond) // Allow goroutine to process
}

func (s *ControlPlaneTestSuite) TestCheckpointing() {
	// Create control plane with only checkpointing enabled
	controlPlane := s.createControlPlane(
		1*time.Hour,         // heartbeat - disabled
		1*time.Hour,         // node info - disabled
		50*time.Millisecond, // checkpoint
	)
	defer s.Require().NoError(controlPlane.Stop(s.ctx))

	// Set sequence number to checkpoint
	s.seqTracker.UpdateLastSeqNum(42)

	// Track checkpoint calls
	var checkpointCalled bool
	s.checkpointer.OnCheckpointSet(func(name string, value uint64) {
		s.Equal("test-checkpoint", name)
		s.Equal(uint64(42), value)
		checkpointCalled = true
	})

	s.Require().NoError(controlPlane.Start(s.ctx))
	// Wait for checkpoint to be called
	s.Eventually(func() bool {
		return checkpointCalled
	}, 100*time.Millisecond, 10*time.Millisecond)

	// Verify checkpoint was stored
	value, err := s.checkpointer.GetStoredCheckpoint("test-checkpoint")
	s.Require().NoError(err)
	s.Equal(uint64(42), value)
}

func (s *ControlPlaneTestSuite) TestErrorHandling() {
	// Create control plane with only heartbeat enabled
	controlPlane := s.createControlPlane(
		50*time.Millisecond, // heartbeat
		1*time.Hour,         // node info - disabled
		1*time.Hour,         // checkpoint - disabled
	)
	defer s.Require().NoError(controlPlane.Stop(s.ctx))

	// Setup error response
	s.requester.EXPECT().
		Request(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("network error")).
		Times(1)

	// Start control plane
	s.Require().NoError(controlPlane.Start(s.ctx))
	time.Sleep(70 * time.Millisecond)

	// Verify health tracker reflects failure
	health := s.healthTracker.GetHealth()
	s.Require().Zero(health.LastSuccessfulHeartbeat)
}
