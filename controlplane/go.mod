module github.com/demo/envoy-xds-demo/controlplane

go 1.22

require (
	github.com/demo/envoyviz v0.0.0
	github.com/envoyproxy/go-control-plane v0.12.0
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.32.0
)

replace github.com/demo/envoyviz => ../visualization

require (
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cncf/xds/go v0.0.0-20230607035331-e9ce68804cb4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/goccy/go-yaml v1.15.13 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
)
