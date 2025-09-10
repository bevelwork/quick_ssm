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
)

type InstanceInfo struct {
	ID          string
	Name        string
	DisplayName string
}

func main() {
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("-- SSM Quick Connect --")
	fmt.Println(strings.Repeat("-", 40))

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

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
