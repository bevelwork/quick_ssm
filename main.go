package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"

	// "github.com/aws/aws-sdk-go-v2/aws"
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
	for range 40 {
		fmt.Print("-")
	}
	fmt.Println()
	fmt.Println("-- SSM Quick Connect --")

	for range 40 {
		fmt.Print("-")
	}
	fmt.Println()
	ctx := context.TODO()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ec2Client := ec2.NewFromConfig(cfg)

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

	instanceNames := map[string]string{}
	for _, r := range reservations {
		for _, i := range r.Instances {
			instanceNames[*i.InstanceId] = "Unknown"

			for _, tag := range i.Tags {
				if *tag.Key == "Name" {
					instanceNames[*i.InstanceId] = *tag.Value
					break
				}
			}
		}
	}

	instanceIDs := []string{}
	for id := range instanceNames {
		instanceIDs = append(instanceIDs, id)
	}
	sort.Strings(instanceIDs)
	nameCount := map[string]int{}
	namesSeen := []string{}
	for _, id := range instanceIDs {
		nameCount[instanceNames[id]]++
		if nameCount[instanceNames[id]] > 1 {
			instanceNames[id] = fmt.Sprintf("%s (%d)", instanceNames[id], nameCount[instanceNames[id]])
		}
		namesSeen = append(namesSeen, instanceNames[id])
	}

	sort.Strings(namesSeen)
	longestName := 0
	for _, name := range namesSeen {
		if len(name) > longestName {
			longestName = len(name)
		}
	}

	for idx, name := range namesSeen {
		instanceID := ""
		for id, instName := range instanceNames {
			if name == instName {
				instanceID = id
				break
			}
		}
		// Thre pading for digit
		fmt.Printf("%3d. %-*s %s\n", idx+1, longestName, name, instanceID)
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
		instanceNames[instanceIDs[inputInt-1]], instanceIDs[inputInt-1],
	)
}
