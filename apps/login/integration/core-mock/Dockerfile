FROM bufbuild/buf:1.54.0 AS proto-files
RUN buf export https://github.com/envoyproxy/protoc-gen-validate.git --path validate --output /proto-files && \
    buf export https://github.com/grpc-ecosystem/grpc-gateway.git --path protoc-gen-openapiv2 --output /proto-files && \
    buf export https://github.com/googleapis/googleapis.git --path protos/zitadelgoogle/api/annotations.proto --path google/api/http.proto --path google/api/field_behavior.proto --output /proto-files && \
    buf export https://github.com/zitadel/zitadel.git --path ./proto/zitadel --output /proto-files

FROM golang:1.20.5-alpine3.18 AS mock-zitadel

RUN go install github.com/eliobischof/grpc-mock/cmd/grpc-mock@01b09f60db1b501178af59bed03b2c22661df48c

COPY mocked-services.cfg .
COPY initial-stubs initial-stubs
COPY --from=proto-files /proto-files/ ./

ENTRYPOINT [ "sh", "-c", "grpc-mock -v 1 -proto $(tr '\n' ',' < ./mocked-services.cfg) -stub-dir ./initial-stubs -mock-addr :22222" ]
