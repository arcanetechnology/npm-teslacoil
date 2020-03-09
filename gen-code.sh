set -e
set -u
set -o pipefail

GRPC_GATEWAY_VERSION="v1.12.2"
PROTOBUF_VERSION="v1.3.1"
PROTO_DIR=.

# Path to this plugin 
PROTOC_GEN_TS_PATH="./node_modules/.bin/protoc-gen-ts"
 
# Directory to write generated code to (.js and .d.ts files) 
OUT_DIR="./src"
 
protoc -I/usr/local/include -I"$PROTO_DIR" \
  -I"$GOPATH/src" \
  -I"$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@$GRPC_GATEWAY_VERSION/third_party/googleapis" \
  -I"$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@$GRPC_GATEWAY_VERSION/" \
  -I"$GOPATH/pkg/mod/github.com/gogo/protobuf@$PROTOBUF_VERSION/gogoproto" \
  --plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" \
  --js_out="import_style=commonjs,binary:${OUT_DIR}" \
  --ts_out="${OUT_DIR}" \
  teslacoil.proto