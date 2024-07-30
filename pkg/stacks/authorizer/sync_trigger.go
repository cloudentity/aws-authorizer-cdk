package authorizer

import (
	"fmt"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	"github.com/aws/jsii-runtime-go"
)

func triggerLambdaInIntervals(stack awscdk.Stack, lambda awslambda.Function, props StackProps) {
	var (
		sqsQueue     awssqs.Queue
		stateMachine awsstepfunctions.StateMachine
	)

	if props.ReloadInterval >= 1*time.Minute {
		createDirectEventBridgeRule(stack, lambda)
		return
	}

	sqsQueue = createSQSQueue(stack)
	stateMachine = createStateMachine(stack, sqsQueue, props)
	createEventBridgeRule(stack, stateMachine)

	lambda.AddEventSource(awslambdaeventsources.NewSqsEventSource(sqsQueue, &awslambdaeventsources.SqsEventSourceProps{
		BatchSize: jsii.Number(1),
	}))
}

func createDirectEventBridgeRule(stack awscdk.Stack, lambda awslambda.Function) {
	rule := awsevents.NewRule(stack, jsii.String("Run Sync Lambda"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(EventBridgeTriggerIntervalMinutes))),
	})
	rule.AddTarget(awseventstargets.NewLambdaFunction(lambda, &awseventstargets.LambdaFunctionProps{}))
}

func createSQSQueue(stack awscdk.Stack) awssqs.Queue {
	deadLetterQueue := awssqs.NewQueue(stack, jsii.String("DeadLetterQueue"), &awssqs.QueueProps{
		RetentionPeriod: awscdk.Duration_Minutes(jsii.Number(1)),
		RemovalPolicy:   awscdk.RemovalPolicy_DESTROY,
	})

	return awssqs.NewQueue(stack, jsii.String("SQSQueue"), &awssqs.QueueProps{
		VisibilityTimeout: awscdk.Duration_Seconds(jsii.Number(30)),
		DeadLetterQueue: &awssqs.DeadLetterQueue{
			Queue:           deadLetterQueue,
			MaxReceiveCount: jsii.Number(1),
		},
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})
}

func createStateMachine(stack awscdk.Stack, queue awssqs.Queue, props StackProps) awsstepfunctions.StateMachine {
	var (
		seconds  = int(props.ReloadInterval.Seconds())
		count    = EventBridgeTriggerIntervalMinutes * 60 / seconds
		sqsTasks = make([]awsstepfunctionstasks.SqsSendMessage, count)
	)
	for i := 0; i < count; i++ {
		sqsTasks[i] = awsstepfunctionstasks.NewSqsSendMessage(stack, jsii.String(fmt.Sprintf("Send Delayed SQS Trigger Message - %d seconds", i*seconds)), &awsstepfunctionstasks.SqsSendMessageProps{
			MessageBody: awsstepfunctions.TaskInput_FromText(jsii.String("Sync")),
			Queue:       queue,
			Delay:       awscdk.Duration_Seconds(jsii.Number(i * seconds)),
		})
	}

	definition := awsstepfunctions.Chain_Start(sqsTasks[0])
	for _, sqsTask := range sqsTasks[1:] {
		definition = definition.Next(sqsTask)
	}

	return awsstepfunctions.NewStateMachine(stack, jsii.String("Sync Looper"), &awsstepfunctions.StateMachineProps{
		DefinitionBody: awsstepfunctions.ChainDefinitionBody_FromChainable(definition),
		RemovalPolicy:  awscdk.RemovalPolicy_DESTROY,
	})
}

func createEventBridgeRule(stack awscdk.Stack, syncLooper awsstepfunctions.StateMachine) {
	rule := awsevents.NewRule(stack, jsii.String("Run Step Function"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(EventBridgeTriggerIntervalMinutes))),
	})
	rule.AddTarget(awseventstargets.NewSfnStateMachine(syncLooper, &awseventstargets.SfnStateMachineProps{}))
	rule.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)
}
