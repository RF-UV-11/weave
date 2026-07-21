package mongodb

// ColNames is the single registry of MongoDB collection names. Every
// mongodb/<collection>.go file and initialize.go reference a collection only
// through this struct — never a raw string literal — so a rename is a one-line change.
var ColNames = struct {
	Ticket string
}{
	Ticket: "tickets",
}
