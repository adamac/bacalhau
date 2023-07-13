package selection

import (
	"context"
	"sort"

	"github.com/bacalhau-project/bacalhau/pkg/lib/math"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/requester"
	"github.com/bacalhau-project/bacalhau/pkg/util/generic"
	"github.com/rs/zerolog/log"
)

type AnyNodeSelectorParams struct {
	NodeDiscoverer       requester.NodeDiscoverer
	NodeRanker           requester.NodeRanker
	OverAskForBidsFactor uint
}

type anyNodeSelector struct {
	nodeDiscoverer       requester.NodeDiscoverer
	nodeRanker           requester.NodeRanker
	overAskForBidsFactor uint
}

// AnyNodeSelector implements a node selection algorithm that will run a job
// across a set of matching nodes up to a user-configurable limit (called the
// "concurrency"). Nodes will be asked to bid in order of rank until at most
// a "concurrency" number of successful executions is reached.
func NewAnyNodeSelector(params AnyNodeSelectorParams) requester.NodeSelector {
	return &anyNodeSelector{
		nodeDiscoverer:       params.NodeDiscoverer,
		nodeRanker:           params.NodeRanker,
		overAskForBidsFactor: math.Max(1, params.OverAskForBidsFactor),
	}
}

func (s *anyNodeSelector) Select(ctx context.Context, job *model.Job, minCount, desiredCount int) ([]model.NodeInfo, error) {
	ctx = log.Ctx(ctx).With().Str("JobID", job.ID()).Logger().WithContext(ctx)

	nodeIDs, err := s.nodeDiscoverer.FindNodes(ctx, *job)
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Debug().Int("Discovered", len(nodeIDs)).Msg("Found nodes for job")

	rankedNodes, err := s.nodeRanker.RankNodes(ctx, *job, nodeIDs)
	if err != nil {
		return nil, err
	}

	// filter nodes with rank below 0
	var filteredNodes []requester.NodeRank
	for _, nodeRank := range rankedNodes {
		if nodeRank.MeetsRequirement() {
			filteredNodes = append(filteredNodes, nodeRank)
		}
	}
	log.Ctx(ctx).Debug().Int("Ranked", len(filteredNodes)).Msg("Ranked nodes for job")

	if len(filteredNodes) < minCount {
		err = requester.NewErrNotEnoughNodes(minCount, rankedNodes)
		return nil, err
	}

	sort.Slice(filteredNodes, func(i, j int) bool {
		return filteredNodes[i].Rank > filteredNodes[j].Rank
	})

	selectedNodes := filteredNodes[:math.Min(len(filteredNodes), desiredCount)]
	selectedInfos := generic.Map(selectedNodes, func(nr requester.NodeRank) model.NodeInfo { return nr.NodeInfo })
	return selectedInfos, nil
}

func (s *anyNodeSelector) SelectNodes(ctx context.Context, job *model.Job) ([]model.NodeInfo, error) {
	minCount := job.Spec.Deal.GetConcurrency()
	desiredCount := minCount * int(s.overAskForBidsFactor)
	return s.Select(ctx, job, minCount, desiredCount)
}

func (s *anyNodeSelector) SelectNodesForRetry(ctx context.Context, job *model.Job, jobState *model.JobState) ([]model.NodeInfo, error) {
	// calculate how many executions we still need, and evaluate if we can ask more nodes to bid
	minExecutions := job.Spec.Deal.GetConcurrency()
	if jobState.NonDiscardedCount() >= minExecutions {
		// There are enough running executions, so no action to take right now.
		return []model.NodeInfo{}, nil
	}

	desiredNodeCount := minExecutions - jobState.NonDiscardedCount()
	return s.Select(ctx, job, desiredNodeCount, desiredNodeCount)
}

// We have over-asked for bids, so now only select the number of nodes we
// actually need.
func (*anyNodeSelector) SelectBids(ctx context.Context, job *model.Job, jobState *model.JobState) (accept, reject []model.ExecutionState) {
	executionsWaiting := jobState.GroupExecutionsByState()[model.ExecutionStateAskForBidAccepted]
	requiredNewExecutions := math.Min(len(executionsWaiting), math.Max(0, job.Spec.Deal.GetConcurrency()-jobState.ActiveCount()))
	return executionsWaiting[:requiredNewExecutions], executionsWaiting[requiredNewExecutions:]
}

func (*anyNodeSelector) CanCompleteJob(ctx context.Context, job *model.Job, jobState *model.JobState) (bool, model.JobStateType) {
	if jobState.CompletedCount() >= job.Spec.Deal.GetConcurrency() {
		return true, model.JobStateCompleted
	}
	return false, jobState.State
}
