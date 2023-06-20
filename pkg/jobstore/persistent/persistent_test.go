//go:build unit || !integration

package persistent

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/bacalhau-project/bacalhau/pkg/jobstore"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PersistentTestSuite struct {
	suite.Suite
	store *PersistentJobStore
	ctx   context.Context
}

func TestPersistentTestSuite(t *testing.T) {
	suite.Run(t, new(PersistentTestSuite))
}

func (s *PersistentTestSuite) TearDownTest() {
	s.store.Close(s.ctx)
}

func (s *PersistentTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.store, _ = NewPersistentJobStore(s.ctx)

	var logicalClock int64 = 0

	jobFixtures := []struct {
		id              string
		totalEntries    int
		jobStates       []model.JobStateType
		executionStates []model.ExecutionStateType
	}{
		{
			id:              "1",
			jobStates:       []model.JobStateType{model.JobStateQueued, model.JobStateInProgress, model.JobStateCancelled},
			executionStates: []model.ExecutionStateType{model.ExecutionStateAskForBid, model.ExecutionStateAskForBidAccepted, model.ExecutionStateFailed, model.ExecutionStateCanceled},
		},
	}

	for _, fixture := range jobFixtures {
		for i, state := range fixture.jobStates {
			oldState := model.JobStateNew
			if i > 0 {
				oldState = fixture.jobStates[i-1]
			}

			jobState := model.JobState{
				JobID:      fixture.id,
				State:      state,
				UpdateTime: time.Unix(logicalClock, 0),
			}
			s.store.appendJobHistory(s.ctx, jobState, oldState, "")
			logicalClock += 1
		}

		for i, state := range fixture.executionStates {
			oldState := model.ExecutionStateNew
			if i > 0 {
				oldState = fixture.executionStates[i-1]
			}

			e := model.ExecutionState{

				JobID:      fixture.id,
				State:      state,
				UpdateTime: time.Unix(logicalClock, 0),
			}
			s.store.appendExecutionHistory(s.ctx, e, oldState, "")
			logicalClock += 1
		}
	}

}

func (s *PersistentTestSuite) TestUnfilteredJobHistory() {
	history, err := s.store.GetJobHistory(s.ctx, "1", jobstore.JobHistoryFilterOptions{})
	require.NoError(s.T(), err, "failed to get job history")
	require.Equal(s.T(), 7, len(history))
}

func (s *PersistentTestSuite) TestJobHistoryOrdering() {
	history, err := s.store.GetJobHistory(s.ctx, "1", jobstore.JobHistoryFilterOptions{})
	require.NoError(s.T(), err, "failed to get job history")
	require.Equal(s.T(), 7, len(history))

	values := make([]int64, len(history))
	for i, h := range history {
		values[i] = h.Time.Unix()
	}

	require.Equal(s.T(), []int64{0, 1, 2, 3, 4, 5, 6}, values)
}

func (s *PersistentTestSuite) TestTimeFilteredJobHistory() {
	options := jobstore.JobHistoryFilterOptions{
		Since: 3,
	}

	history, err := s.store.GetJobHistory(s.ctx, "1", options)
	require.NoError(s.T(), err, "failed to get job history")
	require.Equal(s.T(), 4, len(history))
}

func (s *PersistentTestSuite) TestLevelFilteredJobHistory() {
	jobOptions := jobstore.JobHistoryFilterOptions{
		ExcludeExecutionLevel: true,
	}
	execOptions := jobstore.JobHistoryFilterOptions{
		ExcludeJobLevel: true,
	}

	history, err := s.store.GetJobHistory(s.ctx, "1", jobOptions)
	require.NoError(s.T(), err, "failed to get job history")
	require.Equal(s.T(), 3, len(history))
	require.Equal(s.T(), model.JobStateQueued, history[0].JobState.New)

	history, err = s.store.GetJobHistory(s.ctx, "1", execOptions)
	require.NoError(s.T(), err, "failed to get job history")
	require.Equal(s.T(), 4, len(history))
	require.Equal(s.T(), model.ExecutionStateAskForBid, history[0].ExecutionState.New)
}

func (s *PersistentTestSuite) TestActiveJobs() {

	for i := 0; i < 50; i++ {
		job := model.Job{
			Metadata: model.Metadata{
				ID: strconv.Itoa(i),
			},
		}
		execution := model.ExecutionState{
			JobID: job.ID(),
			State: model.ExecutionStateAskForBid,
		}

		err := s.store.CreateJob(s.ctx, job)
		require.NoError(s.T(), err)

		err = s.store.CreateExecution(s.ctx, execution)
		require.NoError(s.T(), err)
	}

	infos, err := s.store.GetInProgressJobs(s.ctx)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 50, len(infos))
	require.Equal(s.T(), "0", infos[0].Job.ID())
}