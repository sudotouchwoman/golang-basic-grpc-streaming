package main

// Stub for connection.Provider implementation
// that handles gRPC connections to the server thus
// gRPC server actually acts like a broker with
// QoS at most once. There then will be at most one
// gRPC connection to a unique serial connection.
// Several WS connections can read/write to it thorough.
type LoggerServiceClientFactory struct{}
