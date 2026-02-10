# E2E Testing Quick Start

## Prerequisites

Before running end-to-end tests, ensure your environment meets the following requirements:

- **Go 1.25.7+**: Required for building the core and plugins.
- **AWS Credentials**: A valid AWS account with read permissions (for Cost Explorer) and resource creation permissions (for infrastructure tests).
- **FinFocus Core**: Installed locally.

## Setup & Execution

1.  **Install Required Plugins**
    The E2E tests rely on the `aws-public` plugin for fallback pricing and `aws-costexplorer` for actual billing data validation.

    ```bash
    # Install public plugin
    finfocus plugin install github.com/rshade/finfocus-plugin-aws-public
    
    # (Optional) Install Cost Explorer plugin if testing actual costs
    finfocus plugin install github.com/rshade/finfocus-plugin-aws-costexplorer
    ```

2.  **Configure Environment**
    Export your AWS credentials.

    ```bash
    export AWS_ACCESS_KEY_ID="testing-key"
    export AWS_SECRET_ACCESS_KEY="testing-secret"
    export AWS_REGION="us-east-1"
    ```

3.  **Run Tests**
    Execute the E2E test suite using the Makefile target.

    ```bash
    make test-e2e
    ```

4.  **View Results**
    Test results are summarized in JSON format.

    ```bash
    cat test-results/e2e-summary.json
    ```

## Common Issues

- **"Plugin not found"**: Ensure you ran `finfocus plugin install` before testing.
- **"Access Denied"**: Verify your AWS credentials have the necessary permissions.
