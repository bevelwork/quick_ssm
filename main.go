// Package main provides a command-line tool for quickly connecting to AWS EC2 instances
// via AWS Systems Manager (SSM) Session Manager. The tool lists all EC2 instances in
// the current AWS account and provides an interactive interface for selecting and
// connecting to instances using SSM sessions.
package main

import (
	"bufio"
	"context"
	"flag"
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
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// InstanceInfo represents an EC2 instance with its metadata for display purposes.
type InstanceInfo struct {
	ID          string // The EC2 instance ID
	Name        string // The instance name from EC2 tags
	DisplayName string // The formatted display name (may include numbering for duplicates)
}

func main() {
	// Confirm that the AWS CLI is installed
	if _, err := exec.LookPath("aws"); err != nil {
		log.Fatal("AWS CLI not found. Please install it and try again. https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html#getting-started-install-instructions")
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	privateMode := flag.Bool("private-mode", false, "Print account information during execution")
	flag.Parse()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}
	stsClient := sts.NewFromConfig(cfg)
	callerIdentity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatal(fmt.Errorf("failed to authenticate with aws: %v", err))
	}

	spacer := strings.Repeat("-", 40)
	header := []string{
		spacer,
		"-- SSM Quick Connect --",
		spacer,
	}
	if !*privateMode {
		header = append(header, fmt.Sprintf(
			"  Account: %s \n  User: %s",
			*callerIdentity.Account, *callerIdentity.Arn,
		))
		header = append(header, spacer)
	}

	fmt.Println(strings.Join(header, "\n"))

	ec2Client := ec2.NewFromConfig(cfg)

	instances, err := getInstances(ctx, ec2Client)
	if err != nil {
		log.Fatal(err)
	}
	longestName := 0
	for _, inst := range instances {
		if len(inst.DisplayName) > longestName {
			longestName = len(inst.DisplayName)
		}
	}

	for i, inst := range instances {
		entry := fmt.Sprintf(
			"%3d. %-*s %s", i+1, longestName, inst.DisplayName, inst.ID,
		)
		fmt.Println(entry)
	}
	fmt.Println(spacer)

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
		instances[inputInt-1].DisplayName, instances[inputInt-1].ID,
	)

	fmt.Println("Connecting to instance. This may take a few moments: ")

	// Start the SSM session using AWS CLI
	if err := startSSMSession(instances[inputInt-1].ID); err != nil {
		log.Fatal("SSM session failed:", err)
	}
}

// getInstances retrieves all EC2 instances from the AWS account and returns them
// as a sorted list of InstanceInfo structs. The function uses pagination to handle
// accounts with large numbers of instances and extracts instance names from EC2 tags.
func getInstances(ctx context.Context, ec2Client *ec2.Client) ([]*InstanceInfo, error) {
	paginator := ec2.NewDescribeInstancesPaginator(
		ec2Client, &ec2.DescribeInstancesInput{},
	)
	instances := []*InstanceInfo{}
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, i := range output.Reservations {
			for _, inst := range i.Instances {
				instances = append(instances, &InstanceInfo{
					ID:   *inst.InstanceId,
					Name: *inst.Tags[0].Value,
				})
			}
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Name == instances[j].Name {
			return instances[i].ID < instances[j].ID
		}
		return instances[i].Name < instances[j].Name
	})
	addInstanceDisplayNames(instances)

	return instances, nil
}

// addInstanceDisplayNames processes a slice of InstanceInfo structs and updates
// the DisplayName field to handle duplicate instance names by appending numbers
// (e.g., "web-server (2)"). Instances with unique names keep their original name.
func addInstanceDisplayNames(instances []*InstanceInfo) {
	countByName := map[string]int{}
	for i := range instances {
		inst := instances[i]
		countByName[inst.Name]++
		if countByName[inst.Name] > 1 {
			inst.DisplayName = fmt.Sprintf("%s (%d)", inst.Name, countByName[inst.Name])
		} else {
			inst.DisplayName = inst.Name
		}
	}
}

// startSSMSession establishes an interactive SSM session to the specified EC2 instance
// using the AWS CLI. The function handles signal interception for graceful shutdown
// and properly manages the subprocess lifecycle. Returns an error if the session
// cannot be established or terminates unexpectedly.
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
