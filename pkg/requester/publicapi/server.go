package publicapi

import (
	"net/http"

	"github.com/bacalhau-project/bacalhau/pkg/jobstore"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	"github.com/bacalhau-project/bacalhau/pkg/requester"
	sync "github.com/bacalhau-project/golang-mutex-tracer"
	"github.com/gorilla/websocket"
)

const (
	APIPrefix     = "requester/"
	ApprovalRoute = "approve"
	VerifyRoute   = "verify"
)

type RequesterAPIServerParams struct {
	APIServer          *publicapi.APIServer
	Requester          requester.Endpoint
	DebugInfoProviders []model.DebugInfoProvider
	JobStore           jobstore.Store
	NodeDiscoverer     orchestrator.NodeDiscoverer
	AutoCert           string
	TLSCert            string
	TLSKey             string
}

type RequesterAPIServer struct {
	apiServer          *publicapi.APIServer
	requester          requester.Endpoint
	debugInfoProviders []model.DebugInfoProvider
	jobStore           jobstore.Store
	nodeDiscoverer     orchestrator.NodeDiscoverer
	// jobId or "" (for all events) -> connections for that subscription
	websockets      map[string][]*websocket.Conn
	websocketsMutex sync.RWMutex

	// TLS details
	autoCert string
	tlsCert  string
	tlsKey   string
}

func NewRequesterAPIServer(params RequesterAPIServerParams) *RequesterAPIServer {
	return &RequesterAPIServer{
		apiServer:          params.APIServer,
		requester:          params.Requester,
		debugInfoProviders: params.DebugInfoProviders,
		jobStore:           params.JobStore,
		nodeDiscoverer:     params.NodeDiscoverer,
		websockets:         make(map[string][]*websocket.Conn),
		autoCert:           params.AutoCert,
		tlsCert:            params.TLSCert,
		tlsKey:             params.TLSKey,
	}
}

func (s *RequesterAPIServer) RegisterAllHandlers() error {
	handlerConfigs := []publicapi.HandlerConfig{
		{Path: "/" + APIPrefix + "list", Handler: http.HandlerFunc(s.list)},
		{Path: "/" + APIPrefix + "nodes", Handler: http.HandlerFunc(s.nodes)},
		{Path: "/" + APIPrefix + "states", Handler: http.HandlerFunc(s.states)},
		{Path: "/" + APIPrefix + "results", Handler: http.HandlerFunc(s.results)},
		{Path: "/" + APIPrefix + "events", Handler: http.HandlerFunc(s.events)},
		{Path: "/" + APIPrefix + "submit", Handler: http.HandlerFunc(s.submit)},
		{Path: "/" + APIPrefix + "cancel", Handler: http.HandlerFunc(s.cancel)},
		{Path: "/" + APIPrefix + "websocket/events", Handler: http.HandlerFunc(s.websocketJobEvents), Raw: true},
		{Path: "/" + APIPrefix + "logs", Handler: http.HandlerFunc(s.logs), Raw: true},
		{Path: "/" + APIPrefix + "debug", Handler: http.HandlerFunc(s.debug)},
	}
	// register URIs at root prefix for backward compatibility before migrating to API versioning
	// we should remove these eventually, or have throttling limits shared across versions
	err := s.apiServer.RegisterHandlers(publicapi.LegacyAPIPrefix, handlerConfigs...)
	if err != nil {
		return err
	}
	return s.apiServer.RegisterHandlers(publicapi.V1APIPrefix, handlerConfigs...)
}
