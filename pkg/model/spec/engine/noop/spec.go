package noop

import (
	_ "embed"

	"github.com/ipfs/go-cid"
	ipldcodec "github.com/ipld/go-ipld-prime/codec/dagjson"
	dslschema "github.com/ipld/go-ipld-prime/schema/dsl"

	"github.com/bacalhau-project/bacalhau/pkg/model/spec"
	"github.com/bacalhau-project/bacalhau/pkg/model/spec/engine"
)

//go:embed spec.ipldsch
var schema []byte

func load() *engine.Schema {
	dsl, err := dslschema.ParseBytes(schema)
	if err != nil {
		panic(err)
	}
	return (*engine.Schema)(dsl)
}

var (
	EngineSchema        *engine.Schema = load()
	EngineType          cid.Cid        = EngineSchema.Cid()
	defaultModelEncoder                = ipldcodec.Encode
	defaultModelDecoder                = ipldcodec.Decode
)

type NoopEngineSpec struct {
	Noop string
}

func (e *NoopEngineSpec) AsSpec() (spec.Engine, error) {
	return engine.Encode(e, defaultModelEncoder, EngineSchema)
}

func Decode(spec spec.Engine) (*NoopEngineSpec, error) {
	return engine.Decode[NoopEngineSpec](spec, defaultModelDecoder)
}