package heimdall

// go modules are not currently detecting dependencies that are defined in
// .proto files, this is a workaround to allow them to be defined and vendored

import (
	// google/api is imported for generating protos
	_ "github.com/gogo/googleapis/google/api"
	// protobuf/gogoproto is imported for generating protos
	_ "github.com/gogo/protobuf/gogoproto"
	// protobuf/proto is imported for generating protos
	_ "github.com/gogo/protobuf/proto"
)
