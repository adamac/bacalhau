//go:build unit || !integration

package test

import (
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/apimodels"

	"context"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (s *ServerSuite) TestNodeList() {
	ctx := context.Background()
	resp, err := s.client.Nodes().List(ctx, &apimodels.ListNodesRequest{})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp)
	require.NotEmpty(s.T(), resp.Nodes)
	require.Equal(s.T(), 2, len(resp.Nodes))
}

func (s *ServerSuite) TestNodeListLabels() {
	ctx := context.Background()
	req1, err := labels.NewRequirement("name", selection.Equals, []string{"node-0"})
	require.NoError(s.T(), err)
	req2, err := labels.NewRequirement("env", selection.Equals, []string{"devstack"})
	require.NoError(s.T(), err)

	resp, err := s.client.Nodes().List(ctx, &apimodels.ListNodesRequest{
		Labels: []labels.Requirement{*req1, *req2},
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp)
	require.NotEmpty(s.T(), resp.Nodes)
	require.Equal(s.T(), 1, len(resp.Nodes))
}
