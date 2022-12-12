# **Basic server-side streaming with gRPC**

In this simple example server fires several messages with a period of 1s and closes the stream afterwards.
Clients can pass useful information in request body.

### **Code generation with protobuf:**

To generate `go` sources from `proto` file:

```bash
cd pkg/logger
protoc --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative *.proto
```

### **Run server and clients:**

Default address of server is `localhost:8080`, but you can change it with
`-a` flag for both client and server. They should match, otherwise client won't reach the server.

To run server and client(s):

```bash
cd cmd/server && go run .
cd cmd/client && go run .
```
