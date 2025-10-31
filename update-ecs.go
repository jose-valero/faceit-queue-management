package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func main() {
	gitHash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		log.Fatal("Failed to get git hash:", err)
	}
	imageTag := strings.TrimSpace(string(gitHash))

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithSharedConfigProfile("cpx-valero"),
	)
	if err != nil {
		log.Fatal("Failed to load AWS config:", err)
	}

	client := ecs.NewFromConfig(cfg)
	imageURI := fmt.Sprintf("506636091874.dkr.ecr.us-east-1.amazonaws.com/dev-faceit-cluster-app:%s", imageTag)

	fmt.Printf("Fetching current task definition...\n")
	describe, err := client.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: stringPtr("dev-faceit-cluster-app"),
	})
	if err != nil {
		log.Fatal("Failed to describe task definition:", err)
	}

	td := describe.TaskDefinition
	for i := range td.ContainerDefinitions {
		td.ContainerDefinitions[i].Image = &imageURI
	}

	fmt.Printf("Creating new task definition with image: %s\n", imageURI)
	register, err := client.RegisterTaskDefinition(context.TODO(), &ecs.RegisterTaskDefinitionInput{
		Family:                  td.Family,
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		RequiresCompatibilities: td.RequiresCompatibilities,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		TaskRoleArn:             td.TaskRoleArn,
		Volumes:                 td.Volumes,
	})
	if err != nil {
		log.Fatal("Failed to register task definition:", err)
	}

	newTaskDef := fmt.Sprintf("%s:%d", *register.TaskDefinition.Family, register.TaskDefinition.Revision)
	fmt.Printf("Registered new task definition: %s\n", newTaskDef)

	list, err := client.ListTasks(context.Background(), &ecs.ListTasksInput{
		Cluster:     aws.String("ECS-faceit-infra"),
		ServiceName: aws.String("faceit-bot"),
	})

	if err != nil {
		log.Fatal("error listing tasks", err)
	}

	fmt.Printf("Updating ECS service...\n")
	_, err = client.UpdateService(context.TODO(), &ecs.UpdateServiceInput{
		Cluster:        stringPtr("ECS-faceit-infra"),
		Service:        stringPtr("faceit-bot"),
		TaskDefinition: &newTaskDef,
	})
	if err != nil {
		log.Fatal("Failed to update ECS service:", err)
	}

	for _, task := range list.TaskArns {
		fmt.Printf("Stopping task: %s\n", task)
		_, err = client.StopTask(context.TODO(), &ecs.StopTaskInput{
			Cluster: aws.String("ECS-faceit-infra"),
			Task:    aws.String(task),
		})
		if err != nil {
			log.Fatal("Failed to stop task:", err)
		}
	}

	fmt.Println("ECS service updated successfully")
}

func stringPtr(s string) *string { return &s }
