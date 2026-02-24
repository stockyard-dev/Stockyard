import * as cdk from "aws-cdk-lib";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ecsPatterns from "aws-cdk-lib/aws-ecs-patterns";

export class StockyardStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string) {
    super(scope, id);
    new ecsPatterns.ApplicationLoadBalancedFargateService(this, "Stockyard", {
      taskImageOptions: {
        image: ecs.ContainerImage.fromRegistry("stockyard/stockyard:latest"),
        containerPort: 4000,
        environment: {
          OPENAI_API_KEY: process.env.OPENAI_API_KEY!,
        },
      },
      publicLoadBalancer: true,
    });
  }
}
