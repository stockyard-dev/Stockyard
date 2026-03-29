# Pulumi Provider for Stockyard

Manage Stockyard infrastructure with Pulumi.

## Configuration

Set environment variables:

```bash
export STOCKYARD_URL=http://localhost:7749
export STOCKYARD_ADMIN_KEY=your-admin-key
```

## Example (Go)

```go
package main

import (
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        // Enable the cost cap module
        _, err := stockyard.NewModule(ctx, "costcap", &stockyard.ModuleArgs{
            Name:    pulumi.String("costcap"),
            Enabled: pulumi.Bool(true),
        })
        if err != nil {
            return err
        }

        // Create a routing rule for short prompts
        _, err = stockyard.NewRoutingRule(ctx, "short-to-mini", &stockyard.RoutingRuleArgs{
            Name:     pulumi.String("short-to-mini"),
            Priority: pulumi.Int(10),
            Condition: pulumi.Map{
                "field": pulumi.String("prompt_length"),
                "op":    pulumi.String("<"),
                "value": pulumi.Int(500),
            },
            Action: pulumi.Map{
                "route_to_model": pulumi.String("gpt-4o-mini"),
            },
        })
        if err != nil {
            return err
        }

        // Create a cost alert
        _, err = stockyard.NewAlert(ctx, "high-cost", &stockyard.AlertArgs{
            Name:      pulumi.String("daily-cost-warning"),
            Metric:    pulumi.String("cost_usd"),
            Condition: pulumi.String(">"),
            Threshold: pulumi.Float64(100),
        })
        if err != nil {
            return err
        }

        // Invite a team member
        _, err = stockyard.NewTeamMember(ctx, "dev-lead", &stockyard.TeamMemberArgs{
            Email: pulumi.String("dev@company.com"),
            Name:  pulumi.String("Dev Lead"),
            Role:  pulumi.String("developer"),
        })
        return err
    })
}
```

## Building

```bash
cd pulumi-stockyard
go build -o pulumi-resource-stockyard
```

## Resources

| Resource | Description |
|----------|-------------|
| `stockyard:Module` | Toggle proxy middleware modules |
| `stockyard:RoutingRule` | Configure smart routing rules |
| `stockyard:Alert` | Create observe alert rules |
| `stockyard:TeamMember` | Manage team member invites |
