Scribble [![GoDoc](https://godoc.org/github.com/boltdb/bolt?status.svg)](http://godoc.org/github.com/sdomino/scribble) [![Go Report Card](https://goreportcard.com/badge/github.com/sdomino/scribble)](https://goreportcard.com/report/github.com/sdomino/scribble)
--------

A tiny JSON database in Golang

### Installation

Install using `go get github.com/sdomino/scribble`.

### Usage

```go
// a new scribble driver, providing the directory where it will be writing to,
// and a qualified logger if desired
db, err := scribble.New(dir, nil)
if err != nil {
  fmt.Println("Error", err)
}

// Write a fish to the database
fish := Fish{}
if err := db.Write("fish", "onefish", fish); err != nil {
  fmt.Println("Error", err)
}

// Read a fish from the database (passing fish by reference)
onefish := Fish{}
if err := db.Read("fish", "onefish", &onefish); err != nil {
  fmt.Println("Error", err)
}

// Read all fish from the database, unmarshaling the response.
records, err := db.ReadAll("fish")
if err != nil {
  fmt.Println("Error", err)
}

fishies := []Fish{}
for _, f := range records {
  fishFound := Fish{}
  if err := json.Unmarshal([]byte(f), &fishFound); err != nil {
    fmt.Println("Error", err)
  }
  fishies = append(fishies, fishFound)
}

// Delete a fish from the database
if err := db.Delete("fish", "onefish"); err != nil {
  fmt.Println("Error", err)
}

// Delete all fish from the database
if err := db.Delete("fish", ""); err != nil {
  fmt.Println("Error", err)
}
```

## Documentation
- Complete documentation is available on [godoc](http://godoc.org/github.com/sdomino/scribble).
- Coverage Report is available on [gocover](https://gocover.io/github.com/sdomino/scribble)

## Todo/Doing
- Support for windows
- Better support for concurrency
- Better support for sub collections
- More methods to allow different types of reads/writes
- More tests (you can never have enough!)

## Distributed coursework extension

This fork adds two distributed-data features:

1. **HTTP peer replication:** successful `Write` and `Delete` operations are
   forwarded to configured peers. Remote operations are applied locally without
   being forwarded again, preventing replication loops.
2. **Snapshot bootstrap/sync:** a node exposes a JSON snapshot endpoint and a
   new or recovering node can merge that snapshot with `SyncFrom`.

The implementation is intentionally small enough to study. It demonstrates
node-to-node communication and replication, but it is not a consensus system.
Concurrent writes use last-arriving-write-wins behavior.

### Run the tests

```powershell
go test ./...
```

### Two-node experiment

Start the replica:

```powershell
go run ./example/distributed -id node-2 -addr :8082 -data ./demo/node-2
```

Start the primary and configure it to replicate to the replica:

```powershell
go run ./example/distributed -id node-1 -addr :8081 -data ./demo/node-1 -peers http://localhost:8082
```

Write through node 1, then read the replicated value from node 2:

```powershell
Invoke-RestMethod -Method Put -Uri http://localhost:8081/notes/demo -ContentType application/json -Body '{"title":"Distributed Scribble","body":"replicated from node-1"}'
Invoke-RestMethod -Method Get -Uri http://localhost:8082/notes/demo
```

Bootstrap a third node from node 1:

```powershell
go run ./example/distributed -id node-3 -addr :8083 -data ./demo/node-3 -sync-from http://localhost:8081
Invoke-RestMethod -Method Get -Uri http://localhost:8083/notes/demo
```

### API additions

```go
db, _ := scribble.New("./data", &scribble.Options{
    NodeID: "node-1",
    Peers: []string{"http://localhost:8082"},
})

http.ListenAndServe(":8081", db.HTTPHandler())

// On a new/recovering node:
_ = db.SyncFrom("http://localhost:8081")
```
