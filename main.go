package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	yaml "sigs.k8s.io/yaml"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	table "github.com/jedib0t/go-pretty/v6/table"
)

// This datastructure collects all the data
//  related to a single input file passed to the tool
type AllObjDetail struct {
	Objects map[string][]ObjDetail
}

// This datastructure helps abstract
// all the details related to a single manifest 
// instead of passing multiple fields all the time
type ObjDetail struct {
	ObjKind string
	ObjName string
	MemReq  string
	MemLim  string
	CpuReq  string
	CpuLim  string
}

func main() {

	//////////////// Reading whole file in one go
	/*
		Reading the whole file in one go. Not optimal when the manifest is really big (like 10k-20k lines big)

		Todov1: use delimiter based reading:
		- https://www.bacancytechnology.com/qanda/golang/reading-a-file-line-by-line-in-go
		- https://stackoverflow.com/questions/1821811/how-to-read-write-from-to-a-file-using-go

	*/
	yamlRawdata, err := os.ReadFile("./combined_dep-sts.yaml")
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
	}
	// fmt.Println("Printing raw data after reading the file: ", yamlRawdata)
	manifests := splitYAML(string(yamlRawdata))

	var computedFileResult *AllObjDetail = &AllObjDetail{}
	computedFileResult.Objects = make(map[string][]ObjDetail)
	var computedObjResult *ObjDetail
	// var computedObjKind string

	for _, manifest := range manifests {
		_, computedObjResult, err = processEachObject([]byte(manifest))
		if err != nil {
			fmt.Println("Facing error whle parsing / computing: ", err)
		} else if computedObjResult == nil {
			// fmt.Printf("Parsed object is of kind: %s and has no relevance in this computation\n", computedObjKind)
		} else {
			// fmt.Println("obj kind: ", computedObjKind, ", cpu req: ", computedObjResult.CpuReq, ", cpu lim: ", computedObjResult.CpuLim, ", memreq: ", computedObjResult.MemReq, ", mem lim: ", computedObjResult.MemLim)
			(*computedFileResult).Objects[computedObjResult.ObjKind] = append((*computedFileResult).Objects[computedObjResult.ObjKind], *computedObjResult)
		}
	}
		renderOutput(computedFileResult) // print tabular summary
	

}

// This method processes each k8s object and
//  stores the resulting data in an object of
// the `ObjDetail` type. 
// It outsources actual resources extraction to a separate fcuntion
// since podspec is common to both deployments and statefulsets 
func processEachObject(yamlRawdata []byte) (string, *ObjDetail, error) {

	type checkObjKind struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	tmpChkObjKind := checkObjKind{}
	if err := yaml.Unmarshal(yamlRawdata, &tmpChkObjKind); err != nil {
		fmt.Println("Error unmarshalling raw data to check object kind. Error: ", err)
	}

	// TODO v2: The following section feels hacky especially when consideing that more items can popup in future. Go read about  interfaces well & other OSS code  and see if this can be improved.
	switch tmpChkObjKind.Kind {
	case "StatefulSet":
		var inputdepl appsv1.StatefulSet = appsv1.StatefulSet{}
		if err := yaml.Unmarshal(yamlRawdata, &inputdepl); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return tmpChkObjKind.Kind, nil, err
		}

		var podTemplSpec v1.PodSpec = inputdepl.Spec.Template.Spec
		computedObjDetail, err := processPodSpec(podTemplSpec, inputdepl.Name, inputdepl.Kind)
		return tmpChkObjKind.Kind, computedObjDetail, err

	case "Deployment":
		var inputdepl appsv1.Deployment = appsv1.Deployment{}
		if err := yaml.Unmarshal(yamlRawdata, &inputdepl); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return tmpChkObjKind.Kind, nil, err
		}

		var podTemplSpec v1.PodSpec = inputdepl.Spec.Template.Spec
		computedObjDetail, err := processPodSpec(podTemplSpec, inputdepl.Name, inputdepl.Kind)
		return tmpChkObjKind.Kind, computedObjDetail, err

	default:
		// fmt.Println("Neither of preexisting object kinds match. Object kind: ", tmpChkObjKind.Kind)
		return tmpChkObjKind.Kind, nil, nil
	}

}

func processPodSpec(podTemplSpec v1.PodSpec, ObjectName string, ObjectKind string) (*ObjDetail, error) {

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

	currObjData := &ObjDetail{
		ObjName: ObjectName,
		ObjKind: ObjectKind,
		CpuReq:  fmt.Sprintf("%v", cpuReq.AsDec()),
		CpuLim:  fmt.Sprintf("%v", cpuLim.AsDec()),
		MemReq:  humanReadable("memory", memReq.Value()),
		MemLim:  humanReadable("memory", memLim.Value()),
	}

	return currObjData, nil

}

func renderOutput(renderData *AllObjDetail) {

	// w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	// fmt.Fprintln(w, "Name\tKind\tCPU\tMem")

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	// t.SetStyle(table.StyleColoredBright)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = true
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

	t.AppendHeader(table.Row{"Kind", "Name", "CPU", "CPU", "Memory", "Memory"}, rowConfigAutoMerge)
	t.AppendHeader(table.Row{"", "", "Request", "Limit", "Request", "Limit"})
	for objkind, objList := range renderData.Objects {

		for _, obj := range objList {
			t.AppendRow(table.Row{objkind, obj.ObjName, obj.CpuReq, obj.CpuLim, obj.MemReq, obj.MemLim})
		}
	}
	t.Render()
}

// receives ineteger in byes and returns it as a human readable string
func humanReadable(qtyType string, size int64) string {
	switch qtyType {
	case "memory":
		units := []string{"B", "Ki", "Mi", "Gi", "Ti", "Pi"}
		var finalQty float64 = float64(size)
		var ct int8 = 0
		for finalQty/1024 > 1 {
			ct += 1
			finalQty = finalQty / 1024
		}
		return strconv.FormatFloat(finalQty, 'f', 3, 64) + units[ct]

	}
	return ""
}

func splitYAML(yamlContent string) []string {
	var manifests []string
	scanner := bufio.NewScanner(strings.NewReader(yamlContent))
	var sb strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "---" {
			manifests = append(manifests, sb.String())
			sb.Reset()
		} else {
			sb.WriteString(line + "\n")
		}
	}

	if sb.Len() > 0 {
		manifests = append(manifests, sb.String())
	}

	return manifests
}
