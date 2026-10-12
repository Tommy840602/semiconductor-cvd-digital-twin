module cvd/metrology

go 1.26.0

require (
	cvd/protocol v0.0.0-00010101000000-000000000000
	github.com/arloliu/go-secs/v2 v2.4.1
	github.com/awcullen/opcua v1.4.0
)

require (
	github.com/djherbis/buffer v1.2.0 // indirect
	github.com/gammazero/deque v1.0.0 // indirect
	github.com/gammazero/workerpool v1.1.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/phsym/console-slog v0.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
)

replace cvd/protocol => ../protocol
