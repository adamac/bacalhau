package publicapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/bacalhau-project/bacalhau/pkg/job"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/handlerwrapper"
	"github.com/bacalhau-project/bacalhau/pkg/system"
)

// submit godoc
//
//	@ID						pkg/requester/publicapi/submit_docker
//	@Summary				Submits a new docker job to the network.
//	@Description.markdown	endpoints_submit_docker
//	@Tags					Job
//	@Accept					json
//	@Produce				json
//	@Param					submitRequest	body		submitRequest	true	" "
//	@Success				200				{object}	submitResponse
//	@Failure				400				{object}	string
//	@Failure				500				{object}	string
//	@Router					/requester/submit [post]
func (s *RequesterAPIServer) submitDocker(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if otherJobID := req.Header.Get("X-Bacalhau-Job-ID"); otherJobID != "" {
		err := fmt.Errorf("rejecting job because HTTP header X-Bacalhau-Job-ID was set")
		publicapi.HTTPError(ctx, res, err, http.StatusBadRequest)
		return
	}

	payload, err := publicapi.UnmarshalSigned[model.DockerJobCreatePayload](ctx, req.Body)
	if err != nil {
		publicapi.HTTPError(ctx, res, err, http.StatusBadRequest)
		return
	}

	if err := job.VerifyDockerJobCreatePayload(ctx, &payload); err != nil {
		publicapi.HTTPError(ctx, res, err, http.StatusBadRequest)
		return
	}

	jobSpec, err := payload.DockerJob.ToSpec()
	if err != nil {
		publicapi.HTTPError(ctx, res, err, http.StatusBadRequest)
		return
	}

	jobCreatePayload := model.JobCreatePayload{
		ClientID:   payload.ClientID,
		APIVersion: payload.DockerJob.APIVersion.String(),
		Spec:       jobSpec,
	}

	j, err := s.requester.SubmitJob(ctx, jobCreatePayload)
	res.Header().Set(handlerwrapper.HTTPHeaderJobID, j.Metadata.ID)
	ctx = system.AddJobIDToBaggage(ctx, j.Metadata.ID)
	system.AddJobIDFromBaggageToSpan(ctx, oteltrace.SpanFromContext(ctx))

	if err != nil {
		publicapi.HTTPError(ctx, res, err, http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
	err = json.NewEncoder(res).Encode(submitResponse{Job: j})
	if err != nil {
		publicapi.HTTPError(ctx, res, err, http.StatusInternalServerError)
		return
	}
}