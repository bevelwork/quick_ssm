package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("-- SSM Quick Connect --")
	fmt.Println(strings.Repeat("-", 40))

	ctx := context.TODO()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ec2Client := ec2.NewFromConfig(cfg)

	reservations := getInstReservations(ec2Client, ctx)

	instanceIDs, instanceNames, printOut := getInstNameInfo(reservations)
	fmt.Println(strings.Join(printOut, "\n"))

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Select instance. Blank, or non-numeric input will exit: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	input = input[:len(input)-1]
	if input == "" {
		fmt.Println("Exiting")
		return
	}
	inputInt, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Non-numeric input. Exiting")
		return
	}
	fmt.Printf(
		"Selected instance: %s %s\n",
		instanceNames[instanceIDs[inputInt-1]], instanceIDs[inputInt-1],
	)

	fmt.Println("Connecting to instance. This may take a few moments: ")

	// Start the SSM session using AWS CLI
	if err := startSSMSession(instanceIDs[inputInt-1]); err != nil {
		log.Fatal("SSM session failed:", err)
	}
}

// Get instance names, and a list of all names seen, and the longest name
func getInstNameInfo(reservations []*types.Reservation) ([]string, map[string]string, []string) {
	instanceNames := map[string]string{}
	instanceIDs := []string{}
	namesSeen := []string{}
	for _, r := range reservations {
		for _, i := range r.Instances {
			instanceIDs = append(instanceIDs, *i.InstanceId)
			name := "Unknown"

			for _, tag := range i.Tags {
				if *tag.Key == "Name" {
					name = *tag.Value
				}
			}
			instanceNames[*i.InstanceId] = name
			namesSeen = append(namesSeen, name)
		}
	}
	sort.Strings(instanceIDs)
	nameCount := map[string]int{}
	for _, id := range instanceIDs {
		nameCount[instanceNames[id]]++
		if nameCount[instanceNames[id]] > 1 {
			instanceNames[id] = fmt.Sprintf("%s (%d)", instanceNames[id], nameCount[instanceNames[id]])
		}
	}

	sort.Strings(namesSeen)
	longestName := 0
	for _, name := range namesSeen {
		if len(name) > longestName {
			longestName = len(name)
		}
	}
	printout := make([]string, 0, len(namesSeen))
	for idx, name := range namesSeen {
		instanceID := ""
		for id, instName := range instanceNames {
			if name == instName {
				instanceID = id
				break
			}
		}
		// Thre pading for digit
		printout = append(
			printout,
			fmt.Sprintf("%3d. %-*s %s\n", idx+1, longestName, name, instanceID),
		)
	}
	return instanceIDs, instanceNames, printout
}

func getInstReservations(ec2Client *ec2.Client, ctx context.Context) []*types.Reservation {
	reservations := []*types.Reservation{}
	resp, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range resp.Reservations {
		reservations = append(reservations, &r)
	}
	for resp.NextToken != nil {
		resp, err = ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			NextToken: resp.NextToken,
		})
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range resp.Reservations {
			reservations = append(reservations, &r)
		}
	}
	return reservations
}

// Start SSM session using AWS CLI
func startSSMSession(instanceID string) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create the AWS CLI command
	cmd := exec.Command("aws", "ssm", "start-session", "--target", instanceID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start SSM session: %v", err)
	}

	// Wait for the process to complete or for a signal
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		log.Println("Received interrupt signal, terminating SSM session...")
		cmd.Process.Signal(syscall.SIGINT)
		<-done // Wait for the process to exit
	case err := <-done:
		if err != nil {
			return fmt.Errorf("SSM session ended with error: %v", err)
		}
	}

	return nil
}
