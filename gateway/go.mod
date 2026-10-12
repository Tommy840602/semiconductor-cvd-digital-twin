module cvd/gateway

go 1.26.0

require (
	cvd/protocol v0.0.0-00010101000000-000000000000
	github.com/arloliu/go-secs/v2 v2.4.1
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/gopcua/opcua v0.6.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/phsym/console-slog v0.3.1 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)

replace cvd/protocol => ../protocol
