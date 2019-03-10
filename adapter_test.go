package slack

import "github.com/go-joe/joe"

// compile time test to check if we are implementing the interface.
var _ joe.Adapter = new(API)
