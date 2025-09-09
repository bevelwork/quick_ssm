package main

import (
	"fmt"
)

// This Utility quickly established a connection to an SSM-enabled
// server. It pulls a list of all ec2 instances in the provided aws
// account and region, and prints out friendly names for them.
// The TUI allows the used to select which instance to connect to,
// and creates a TTY session to it.
// In the event of failure we have a --check option that will check
// if the instance has a role associated with it, and other obvious
// aspects that would prevent SSM from working.

func main() {
	fmt.Println("Hello World")
}
