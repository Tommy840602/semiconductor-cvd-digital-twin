module cvd/bridge

go 1.26.0

require (
	cvd/protocol v0.0.0
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/segmentio/kafka-go v0.4.49
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)

replace cvd/protocol => ../protocol
