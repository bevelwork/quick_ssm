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
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ANSI color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// color wraps a string with the specified color code
func color(text, colorCode string) string {
	return colorCode + text + ColorReset
}

// colorBold wraps a string with the specified color code and bold formatting
func colorBold(text, colorCode string) string {
	return colorCode + ColorBold + text + ColorReset
}

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
	privateMode := flag.Bool("private-mode", false, "Hide account information during execution")
	checkMode := flag.Bool("check", false, "Perform diagnostic checks on the selected instance")
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
	if *checkMode {
		header = append(header, colorBold("<> <> DIAGNOSTIC MODE <> <>", ColorCyan))
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
		// Alternate row colors for better readability
		var rowColor string
		if i%2 == 0 {
			rowColor = ColorWhite // Default color for even rows
		} else {
			rowColor = ColorCyan // Subtle cyan for odd rows
		}

		entry := fmt.Sprintf(
			"%3d. %-*s %s", i+1, longestName, inst.DisplayName, inst.ID,
		)
		fmt.Println(color(entry, rowColor))
	}
	fmt.Println(color(spacer, ColorBlue))

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s", color("Select instance. Blank, or non-numeric input will exit: ", ColorYellow))
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
	selectedInstance := instances[inputInt-1]
	fmt.Printf(
		"Selected instance: %s %s\n",
		colorBold(selectedInstance.DisplayName, ColorGreen),
		color(selectedInstance.ID, ColorWhite),
	)

	if *checkMode {
		// Perform diagnostic checks
		ec2Client := ec2.NewFromConfig(cfg)
		iamClient := iam.NewFromConfig(cfg)
		if err := performDiagnostics(ctx, ec2Client, iamClient, selectedInstance.ID); err != nil {
			log.Fatal("Diagnostic check failed:", err)
		}
		return
	}

	fmt.Println("Connecting to instance. This may take a few moments: ")

	// Start the SSM session using AWS CLI
	if err := startSSMSession(selectedInstance.ID); err != nil {
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
				instanceName := "unknown"
				// Look for the "Name" tag specifically
				for _, tag := range inst.Tags {
					if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
						instanceName = *tag.Value
						break
					}
				}

				instances = append(instances, &InstanceInfo{
					ID:   *inst.InstanceId,
					Name: instanceName,
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

// DiagnosticResult represents the result of a diagnostic check
type DiagnosticResult struct {
	CheckName string
	Status    string // "PASS", "FAIL", "WARN"
	Message   string
}

// performDiagnostics runs comprehensive diagnostic checks on the specified instance
// including IAM role attachment, internet connectivity, and SSM traffic requirements.
func performDiagnostics(ctx context.Context, ec2Client *ec2.Client, iamClient *iam.Client, instanceID string) error {
	fmt.Printf("\n%s\n", color(strings.Repeat("=", 60), ColorBlue))
	fmt.Printf("%s\n", colorBold("DIAGNOSTIC CHECKS FOR INSTANCE: "+color(instanceID, ColorWhite), ColorBlue))
	fmt.Printf("%s\n", color(strings.Repeat("=", 60), ColorBlue))

	var results []DiagnosticResult

	// Get instance details
	instance, err := getInstanceDetails(ctx, ec2Client, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance details: %v", err)
	}

	// Check 1: IAM Role Attachment
	iamResult := checkIAMRole(ctx, iamClient, instance)
	results = append(results, iamResult)

	// Check 2: Internet Connectivity
	internetResult := checkInternetConnectivity(ctx, ec2Client, instance)
	results = append(results, internetResult)

	// Check 3: SSM Traffic Rules
	ssmResult := checkSSMTrafficRules(ctx, ec2Client, instance)
	results = append(results, ssmResult)

	// Display results
	displayDiagnosticResults(results)

	return nil
}

// getInstanceDetails retrieves detailed information about a specific EC2 instance
func getInstanceDetails(ctx context.Context, ec2Client *ec2.Client, instanceID string) (*types.Instance, error) {
	result, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return &result.Reservations[0].Instances[0], nil
}

// checkIAMRole verifies if the instance has an IAM role attached with SSM permissions
func checkIAMRole(ctx context.Context, iamClient *iam.Client, instance *types.Instance) DiagnosticResult {
	if instance.IamInstanceProfile == nil || instance.IamInstanceProfile.Arn == nil {
		return DiagnosticResult{
			CheckName: "IAM Role Attachment",
			Status:    "FAIL",
			Message:   "No IAM instance profile attached to the instance",
		}
	}

	// Extract role name from ARN
	arn := *instance.IamInstanceProfile.Arn
	roleName := extractRoleNameFromProfileArn(arn)
	if roleName == "" {
		return DiagnosticResult{
			CheckName: "IAM Role Attachment",
			Status:    "WARN",
			Message:   fmt.Sprintf("IAM profile attached but could not extract role name from ARN: %s", arn),
		}
	}

	// Check if role has SSM permissions
	hasSSMPermissions, err := checkRoleSSMPermissions(ctx, iamClient, roleName)
	if err != nil {
		return DiagnosticResult{
			CheckName: "IAM Role Attachment",
			Status:    "WARN",
			Message:   fmt.Sprintf("IAM role '%s' attached but could not verify SSM permissions: %v", roleName, err),
		}
	}

	if hasSSMPermissions {
		return DiagnosticResult{
			CheckName: "IAM Role Attachment",
			Status:    "PASS",
			Message:   fmt.Sprintf("IAM role '%s' attached with SSM permissions", roleName),
		}
	}

	return DiagnosticResult{
		CheckName: "IAM Role Attachment",
		Status:    "FAIL",
		Message:   fmt.Sprintf("IAM role '%s' attached but missing required SSM permissions", roleName),
	}
}

// checkInternetConnectivity verifies if the instance's subnet has internet access
func checkInternetConnectivity(ctx context.Context, ec2Client *ec2.Client, instance *types.Instance) DiagnosticResult {
	if instance.SubnetId == nil {
		return DiagnosticResult{
			CheckName: "Internet Connectivity",
			Status:    "FAIL",
			Message:   "Instance has no subnet ID",
		}
	}

	// Get subnet details
	subnetResult, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{*instance.SubnetId},
	})
	if err != nil {
		return DiagnosticResult{
			CheckName: "Internet Connectivity",
			Status:    "WARN",
			Message:   fmt.Sprintf("Could not retrieve subnet details: %v", err),
		}
	}

	if len(subnetResult.Subnets) == 0 {
		return DiagnosticResult{
			CheckName: "Internet Connectivity",
			Status:    "FAIL",
			Message:   "Subnet not found",
		}
	}

	subnet := subnetResult.Subnets[0]
	vpcID := *subnet.VpcId

	// Check route table for internet gateway route
	routeTables, err := ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   stringPtr("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		return DiagnosticResult{
			CheckName: "Internet Connectivity",
			Status:    "WARN",
			Message:   fmt.Sprintf("Could not check route tables: %v", err),
		}
	}

	hasInternetRoute := false
	for _, rt := range routeTables.RouteTables {
		// Check if this route table is associated with the subnet
		associatedWithSubnet := false
		for _, assoc := range rt.Associations {
			if assoc.SubnetId != nil && *assoc.SubnetId == *instance.SubnetId {
				associatedWithSubnet = true
				break
			}
		}

		if associatedWithSubnet {
			// Check for 0.0.0.0/0 route to internet gateway
			for _, route := range rt.Routes {
				if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == "0.0.0.0/0" {
					if route.GatewayId != nil && strings.HasPrefix(*route.GatewayId, "igw-") {
						hasInternetRoute = true
						break
					}
				}
			}
		}
	}

	if hasInternetRoute {
		return DiagnosticResult{
			CheckName: "Internet Connectivity",
			Status:    "PASS",
			Message:   "Subnet has internet gateway route (0.0.0.0/0)",
		}
	}

	return DiagnosticResult{
		CheckName: "Internet Connectivity",
		Status:    "FAIL",
		Message:   "Subnet lacks internet gateway route (0.0.0.0/0) - instance may not have internet access",
	}
}

// checkSSMTrafficRules verifies security group rules allow SSM traffic
func checkSSMTrafficRules(ctx context.Context, ec2Client *ec2.Client, instance *types.Instance) DiagnosticResult {
	if len(instance.SecurityGroups) == 0 {
		return DiagnosticResult{
			CheckName: "SSM Traffic Rules",
			Status:    "FAIL",
			Message:   "Instance has no security groups",
		}
	}

	securityGroupIds := make([]string, len(instance.SecurityGroups))
	for i, sg := range instance.SecurityGroups {
		securityGroupIds[i] = *sg.GroupId
	}

	// Get security group details
	sgResult, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: securityGroupIds,
	})
	if err != nil {
		return DiagnosticResult{
			CheckName: "SSM Traffic Rules",
			Status:    "WARN",
			Message:   fmt.Sprintf("Could not retrieve security group details: %v", err),
		}
	}

	hasHTTPSOutbound := false
	hasAllTrafficOutbound := false

	for _, sg := range sgResult.SecurityGroups {
		// Check outbound rules for SSM traffic
		for _, rule := range sg.IpPermissionsEgress {
			// Check for "All Traffic" (-1) or "All TCP" (6) outbound
			if rule.IpProtocol != nil {
				if *rule.IpProtocol == "-1" || *rule.IpProtocol == "6" {
					if rule.IpRanges != nil {
						for _, ipRange := range rule.IpRanges {
							if ipRange.CidrIp != nil && (*ipRange.CidrIp == "0.0.0.0/0" || *ipRange.CidrIp == "::/0") {
								hasAllTrafficOutbound = true
								break
							}
						}
					}
				}
			}

			// Check for HTTPS (443) outbound
			if rule.FromPort != nil && rule.ToPort != nil {
				if *rule.FromPort <= 443 && *rule.ToPort >= 443 {
					if rule.IpRanges != nil {
						for _, ipRange := range rule.IpRanges {
							if ipRange.CidrIp != nil && (*ipRange.CidrIp == "0.0.0.0/0" || *ipRange.CidrIp == "::/0") {
								hasHTTPSOutbound = true
								break
							}
						}
					}
				}
			}
		}
	}

	if hasAllTrafficOutbound {
		return DiagnosticResult{
			CheckName: "SSM Traffic Rules",
			Status:    "PASS",
			Message:   "Security groups allow all traffic outbound (includes SSM requirements)",
		}
	}

	if hasHTTPSOutbound {
		return DiagnosticResult{
			CheckName: "SSM Traffic Rules",
			Status:    "PASS",
			Message:   "Security groups allow HTTPS outbound traffic (required for SSM)",
		}
	}

	return DiagnosticResult{
		CheckName: "SSM Traffic Rules",
		Status:    "FAIL",
		Message:   "Security groups do not allow HTTPS outbound traffic (required for SSM)",
	}
}

// Helper functions

func extractRoleNameFromProfileArn(arn string) string {
	// ARN format: arn:aws:iam::account:instance-profile/profile-name
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}

func checkRoleSSMPermissions(ctx context.Context, iamClient *iam.Client, roleName string) (bool, error) {
	// Get attached policies
	policies, err := iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return false, err
	}

	// Check for AmazonSSMManagedInstanceCore policy
	for _, policy := range policies.AttachedPolicies {
		if policy.PolicyArn != nil && strings.Contains(*policy.PolicyArn, "AmazonSSMManagedInstanceCore") {
			return true, nil
		}
	}

	// Check inline policies
	inlinePolicies, err := iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return false, err
	}

	for _, policyName := range inlinePolicies.PolicyNames {
		policy, err := iamClient.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
			RoleName:   &roleName,
			PolicyName: &policyName,
		})
		if err != nil {
			continue
		}

		// Basic check for SSM permissions in policy document
		if policy.PolicyDocument != nil && strings.Contains(*policy.PolicyDocument, "ssm:") {
			return true, nil
		}
	}

	return false, nil
}

func stringPtr(s string) *string {
	return &s
}

func displayDiagnosticResults(results []DiagnosticResult) {
	fmt.Println()
	for _, result := range results {
		var statusIcon, colorCode string
		switch result.Status {
		case "PASS":
			statusIcon = "✅"
			colorCode = ColorGreen
		case "FAIL":
			statusIcon = "❌"
			colorCode = ColorRed
		case "WARN":
			statusIcon = "⚠️"
			colorCode = ColorYellow
		default:
			statusIcon = "❓"
			colorCode = ColorWhite
		}

		fmt.Printf("%s %s: %s\n", statusIcon, colorBold(result.CheckName, colorCode), result.Message)
	}

	fmt.Printf("\n%s\n", color(strings.Repeat("=", 60), ColorPurple))
	fmt.Printf("%s\n", colorBold("DIAGNOSTIC SUMMARY", ColorPurple))
	fmt.Printf("%s\n", color(strings.Repeat("=", 60), ColorPurple))

	passCount := 0
	failCount := 0
	warnCount := 0

	for _, result := range results {
		switch result.Status {
		case "PASS":
			passCount++
		case "FAIL":
			failCount++
		case "WARN":
			warnCount++
		}
	}

	fmt.Printf("%s✅ Passed: %s\n", ColorGreen, colorBold(fmt.Sprintf("%d", passCount), ColorGreen))
	fmt.Printf("%s⚠️  Warnings: %s\n", ColorYellow, colorBold(fmt.Sprintf("%d", warnCount), ColorYellow))
	fmt.Printf("%s❌ Failed: %s\n", ColorRed, colorBold(fmt.Sprintf("%d", failCount), ColorRed))

	if failCount == 0 && warnCount == 0 {
		fmt.Printf("\n%s\n", color("🎉 All checks passed! Instance should be ready for SSM connection.", ColorGreen))
	} else if failCount > 0 {
		fmt.Printf("\n%s\n", color("⚠️  Some checks failed. Please address the issues above before connecting.", ColorRed))
	} else {
		fmt.Printf("\n%s\n", color("⚠️  Some warnings detected. Instance may work but review the warnings above.", ColorYellow))
	}
}
