package main

import (
	"fmt"
	"os"

	yaml "sigs.k8s.io/yaml"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func main() {
	//////////////// Reading whole file in one go
	/*
		Reading the whole file in one go. Not optimal when the manifest is really big (like 10k-20k lines big)

		Todov1: use delimiter based reading:
		- https://www.bacancytechnology.com/qanda/golang/reading-a-file-line-by-line-in-go
		- https://stackoverflow.com/questions/1821811/how-to-read-write-from-to-a-file-using-go

	*/
	yamlRawdata, err := os.ReadFile("./sample-dep.yaml")
	if err != nil {
		fmt.Println("Error occurred while tryig to readfile")
	}
	fmt.Println("Printing raw data after reading the file: ", yamlRawdata)

	var inputdepl appsv1.Deployment = appsv1.Deployment{}
	if err := yaml.Unmarshal(yamlRawdata, &inputdepl); err != nil {
		fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
		fmt.Println("priting inputdepl for debugging: ", inputdepl)
		return
	}

	var podTemplSpec v1.PodSpec = inputdepl.Spec.Template.Spec
	var cpuReq, cpuLim, memReq, memLim resource.Quantity = resource.Quantity{}, resource.Quantity{}, resource.Quantity{}, resource.Quantity{} // units of Mi and m
	for _, container := range podTemplSpec.Containers {
		// fmt.Printf("\nreq type: %T", container.Resources.Requests.Cpu())
		cpuReq.Add(*container.Resources.Requests.Cpu())
		cpuLim.Add(*container.Resources.Limits.Cpu())

		memReq.Add(*container.Resources.Requests.Memory())
		memLim.Add(*container.Resources.Limits.Memory())

	}
	for _, container := range podTemplSpec.InitContainers {
		cpuReq.Add(*container.Resources.Requests.Cpu())
		cpuLim.Add(*container.Resources.Limits.Cpu())

		memReq.Add(*container.Resources.Requests.Memory())
		memLim.Add(*container.Resources.Limits.Memory())
	}

	fmt.Println("cpu req: ", cpuReq.String(), ", cpu lim: ", cpuLim.String(), ", memreq: ", memReq.Value(), ", mem lim: ", memLim.Value())
}
