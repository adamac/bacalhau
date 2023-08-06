//go:build unit || !integration

package publicapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/suite"

	"github.com/bacalhau-project/bacalhau/pkg/logger"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/node"
	"github.com/bacalhau-project/bacalhau/pkg/requester/publicapi"
	testutils "github.com/bacalhau-project/bacalhau/pkg/test/utils"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type WebsocketSuite struct {
	suite.Suite
	node   *node.Node
	client *publicapi.RequesterAPIClient
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestWebsocketSuite(t *testing.T) {
	suite.Run(t, new(WebsocketSuite))
}

// Before each test
func (s *WebsocketSuite) SetupTest() {
	logger.ConfigureTestLogging(s.T())
	n, client := setupNodeForTest(s.T())
	s.node = n
	s.client = client
}

// After each test
func (s *WebsocketSuite) TearDownTest() {
	s.node.CleanupManager.Cleanup(context.Background())
}

func (s *WebsocketSuite) TestWebsocketEverything() {
	ctx := context.Background()
	// string.Replace http with ws in c.BaseURI
	url := *s.client.BaseURI
	url.Scheme = "ws"
	wurl := url.JoinPath("requester", "websocket", "events")

	conn, _, err := websocket.DefaultDialer.Dial(wurl.String(), nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		s.NoError(conn.Close())
	})

	eventChan := make(chan model.JobEvent)
	go func() {
		defer close(eventChan)
		for {
			var event model.JobEvent
			err = conn.ReadJSON(&event)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.Require().NoError(err)
			eventChan <- event
		}
	}()

	// Pause to ensure the websocket connects _before_ we submit the job
	time.Sleep(100 * time.Millisecond)

	genericJob := testutils.MakeGenericJob()
	_, err = s.client.Submit(ctx, genericJob)
	s.Require().NoError(err)

	event := <-eventChan
	s.Require().Equal(model.JobEventCreated, event.EventName)

}

func (s *WebsocketSuite) TestWebsocketSingleJob() {
	s.T().Skip("TODO: test is flaky as by the time we connect to the websocket, " +
		"the job has already progressed and first event is not guaranteed to be 'Created'")
	ctx := context.Background()

	genericJob := testutils.MakeGenericJob()
	j, err := s.client.Submit(ctx, genericJob)
	s.Require().NoError(err)

	url := *s.client.BaseURI
	url.Scheme = "ws"
	wurl := url.JoinPath("websocket", "events")
	wurl.RawQuery = fmt.Sprintf("job_id=%s", j.Metadata.ID)

	conn, _, err := websocket.DefaultDialer.Dial(wurl.String(), nil)
	s.Require().NoError(err)

	var event model.JobEvent
	err = conn.ReadJSON(&event)
	s.Require().NoError(err)
	s.Require().Equal("Created", event.EventName.String())
}
