# Quick SSM

A simple Go CLI tool for quickly connecting to AWS EC2 instances via AWS Systems Manager (SSM) Session Manager. This tool lists all your EC2 instances and allows you to select one for an interactive SSM session.

## Features

- Lists all EC2 instances in your AWS account
- Displays instance names and IDs in a numbered menu
- Handles duplicate instance names by adding numbers (e.g., "web-server (2)")
- Sorts instances alphabetically by name
- Provides an interactive selection interface
- Establishes SSM sessions using the AWS CLI
- Supports graceful shutdown with signal handling
- Optional private mode to hide account information

## Prerequisites

### Required Software

1. **Go 1.24.4 or later** - [Download and install Go](https://golang.org/dl/)
2. **AWS CLI** - [Install AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html#getting-started-install-instructions)

### AWS Configuration

1. **AWS Credentials**: Configure your AWS credentials using one of these methods:
   - AWS CLI: `aws configure`
   - Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
   - IAM roles (if running on EC2)
   - AWS credentials file

2. **Required IAM Permissions**: Your AWS credentials need the following permissions:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "ec2:DescribeInstances",
           "ec2:DescribeSubnets",
           "ec2:DescribeRouteTables",
           "ec2:DescribeSecurityGroups",
           "sts:GetCallerIdentity"
         ],
         "Resource": "*"
       },
       {
         "Effect": "Allow",
         "Action": [
           "ssm:StartSession"
         ],
         "Resource": "arn:aws:ec2:*:*:instance/*"
       },
       {
         "Effect": "Allow",
         "Action": [
           "iam:ListAttachedRolePolicies",
           "iam:ListRolePolicies",
           "iam:GetRolePolicy"
         ],
         "Resource": "*"
       }
     ]
   }
   ```

3. **SSM Agent**: Target EC2 instances must have the SSM Agent installed and running. Most modern Amazon Linux, Ubuntu, and Windows AMIs include it by default.

4. **Instance IAM Role**: EC2 instances need an IAM role with the `AmazonSSMManagedInstanceCore` policy attached.

## Installation

### Option 1: Build from Source

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd quick_ssm
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the binary:
   ```bash
   go build -o quick_ssm main.go
   ```

4. (Optional) link the binary to `/usr/local/bin`:
   ```bash
   ln -s $(pwd)/quick_ssm /usr/local/bin/quick_ssm
   ```

### Option 2: Go Install

If the module is available in a Go module repository:

```bash
go install github.com/bevelwork/quick_ssm@latest
```

### Option 3: Download Release

Download the latest release binary from the [Releases page](https://github.com/bevelwork/quick_ssm/releases):

```bash
# Download the latest release
curl -L https://github.com/bevelwork/quick_ssm/releases/latest/download/quick_ssm-v1.0.0-linux-amd64 -o quick_ssm
chmod +x quick_ssm
```

## Usage

### Basic Usage

```bash
./quick_ssm
```

The tool will:
1. Display your AWS account information
2. List all EC2 instances with numbered options
3. Prompt you to select an instance
4. Establish an SSM session to the selected instance

### Command Line Options

```bash
./quick_ssm -h
```

Available options:
- `-private-mode`: Hide account information during execution (useful for screenshots or demos)
- `-check`: Perform diagnostic checks on the selected instance instead of connecting

Examples:
```bash
# Hide account information
./quick_ssm -private-mode

# Run diagnostic checks on selected instance
./quick_ssm -check
```

### Example Session

```
----------------------------------------
-- SSM Quick Connect --
----------------------------------------
  Account: 123456789012 
  User: arn:aws:iam::123456789012:user/your-username
----------------------------------------
  1. web-server-1     i-0123456789abcdef0
  2. web-server-2     i-0fedcba9876543210
  3. database-server  i-0abcdef1234567890
  4. api-server (2)   i-0987654321fedcba0
----------------------------------------
Select instance. Blank, or non-numeric input will exit: 2
Selected instance: web-server-2 i-0fedcba9876543210
Connecting to instance. This may take a few moments: 
```

### Diagnostic Checks

The `--check` flag performs comprehensive diagnostic checks on the selected instance to verify SSM connectivity requirements:

1. **IAM Role Attachment**: Verifies the instance has an IAM role with SSM permissions
2. **Internet Connectivity**: Checks if the instance's subnet has internet access via route table
3. **SSM Traffic Rules**: Validates security group rules allow HTTPS outbound traffic

Example diagnostic output:
```
============================================================
DIAGNOSTIC CHECKS FOR INSTANCE: i-0123456789abcdef0
============================================================

✅ IAM Role Attachment: IAM role 'SSMInstanceRole' attached with SSM permissions
✅ Internet Connectivity: Subnet has internet gateway route (0.0.0.0/0)
✅ SSM Traffic Rules: Security groups allow HTTPS outbound traffic (required for SSM)

============================================================
DIAGNOSTIC SUMMARY
============================================================
✅ Passed: 3
⚠️  Warnings: 0
❌ Failed: 0

🎉 All checks passed! Instance should be ready for SSM connection.
```

### Exiting

- **During selection**: Press Enter (blank input) or enter non-numeric text to exit
- **During SSM session**: Press `Ctrl+C` to gracefully terminate the session

## How It Works

1. **Authentication**: Uses AWS SDK v2 to authenticate with your AWS account
2. **Instance Discovery**: Queries EC2 to get all instances using pagination
3. **Name Resolution**: Extracts instance names from EC2 tags (assumes first tag is the name)
4. **Duplicate Handling**: Adds numbers to duplicate names for clarity
5. **Sorting**: Sorts instances alphabetically by name, then by ID
6. **SSM Connection**: Uses AWS CLI to establish the SSM session
7. **Signal Handling**: Properly handles interrupt signals for clean shutdown

## Troubleshooting

### Common Issues

1. **"AWS CLI not found"**
   - Install AWS CLI following the [official installation guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)

2. **"failed to authenticate with aws"**
   - Run `aws configure` to set up your credentials
   - Verify your credentials with `aws sts get-caller-identity`

3. **"SSM session failed"**
   - Ensure the target instance has SSM Agent installed and running
   - Verify the instance has the required IAM role with SSM permissions
   - Check that the instance is in a subnet with internet access or VPC endpoints for SSM

4. **No instances listed**
   - Verify you have `ec2:DescribeInstances` permissions
   - Check that your instances have the required tags
   - Ensure you're in the correct AWS region

5. **"Instance not found" or connection timeout**
   - Verify the instance is running
   - Check that SSM Agent is running on the instance
   - Ensure network connectivity between your machine and AWS

### Debugging

Enable AWS CLI debug logging:
```bash
export AWS_CLI_FILE_ENCODING=UTF-8
export AWS_CLI_AUTO_PROMPT=off
aws configure set cli_log_level debug
```

## Dependencies

- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
  - EC2 service (for instance discovery and network diagnostics)
  - IAM service (for role permission checks)
  - STS service (for authentication)
- AWS CLI (for SSM session management)

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.

## Version Management

This project uses a simple date-based versioning system: `major.minor.YYYYMMDD`

### Version Format

- **Major.Minor**: Set in `version.yml` file
- **Date**: Automatically appended as YYYYMMDD format
- **Example**: `v1.0.20241201` (version 1.0 released on December 1, 2024)

### Automatic Versioning

- **Main branch pushes**: Automatically create release with current date
- **Manual releases**: Use GitHub Actions "Run workflow" to trigger release
- **Release artifacts**: Binary and checksums are automatically generated

### Manual Version Management

Update the base version in `version.yml` and use the provided scripts:

```bash
# Show current version
make version
# or
./scripts/version.sh current

# Update major version
make major 2
# or
./scripts/version.sh major 2

# Update minor version  
make minor 1
# or
./scripts/version.sh minor 1

# Build release binary locally
make build-release
```

### Release Process

1. **Automatic**: Push to main branch triggers release with current date
2. **Manual**: Use GitHub Actions "Run workflow" to create release
3. **Version Updates**: Edit `version.yml` to change major/minor versions

### Development

```bash
# Build for development
make build

# Run tests
make test

# Format and lint code
make lint

# Clean build artifacts
make clean
```

## Contributing

[Add contribution guidelines here]
