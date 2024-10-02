package estimate

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

	"k8s.io/kubernetes/pkg/apis/autoscaling"

	// "github.com/spf13/cobra"
)

// This datastructure collects all the data
//  related to a single input file passed to the tool
type AllObjDetail struct {
	Objects map[string][]ObjDetail
}

func (a *AllObjDetail) chkIfObjAdded(targetObjKind string, targetObjName string) (*ObjDetail) {

	objects, exists :=  a.Objects[targetObjKind]
	if exists {
		for _, computedObj := range objects  {
			if computedObj.ObjName == targetObjName {
				return &computedObj
			}
		}	
		return nil
	} else {
		return nil
	}	
}

// This datastructure helps abstract
// all the details related to a single manifest 
// instead of passing multiple fields all the time
type ObjDetail struct {
	ObjKind string
	ObjName string
	MemReq  int64
	MemLim  int64
	CpuReq  string
	CpuLim  string
	Replicas int32
	MinReplicas int32
	MaxReplicas int32
}

/////////////////////////////////////////////////////////////////////

func ProcessManifest(manifestPath string, reportVerbosity int) {

	//////////////// Reading whole file in one go
	/*
		Reading the whole file in one go. Not optimal when the manifest is really big (like 10k-20k lines big)

		Todov1: use delimiter based reading:
		- https://www.bacancytechnology.com/qanda/golang/reading-a-file-line-by-line-in-go
		- https://stackoverflow.com/questions/1821811/how-to-read-write-from-to-a-file-using-go

	*/
	yamlRawdata, err := os.ReadFile(manifestPath)
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
		_, computedObjResult, err = processEachObject([]byte(manifest), computedFileResult)

		
		if err != nil {
			fmt.Println("Facing error whle parsing / computing: ", err)
		} else if computedObjResult == nil {
			// fmt.Printf("Parsed object is of kind: %s and has no relevance in this computation\n", computedObjKind)
		} else {
			// fmt.Println("obj kind: ", computedObjKind, ", cpu req: ", computedObjResult.CpuReq, ", cpu lim: ", computedObjResult.CpuLim, ", memreq: ", computedObjResult.MemReq, ", mem lim: ", computedObjResult.MemLim)
			(*computedFileResult).Objects[computedObjResult.ObjKind] = append((*computedFileResult).Objects[computedObjResult.ObjKind], *computedObjResult)
		}
	}
	renderOutput(reportVerbosity, computedFileResult) // print tabular summary
	

}

// This method processes each k8s object and
//  stores the resulting data in an object of
// the `ObjDetail` type. 
// It outsources actual resources extraction to a separate fcuntion
// since podspec is common to both deployments and statefulsets 
func processEachObject(yamlRawdata []byte, computedFileResult *AllObjDetail) (string, *ObjDetail, error) {

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
		var inputManifestObj appsv1.StatefulSet = appsv1.StatefulSet{}
		if err := yaml.Unmarshal(yamlRawdata, &inputManifestObj); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return tmpChkObjKind.Kind, nil, err
		}

		var podTemplSpec v1.PodSpec = inputManifestObj.Spec.Template.Spec
		var replicas int32
		if inputManifestObj.Spec.Replicas != nil {
			replicas = *inputManifestObj.Spec.Replicas
		} else  {
			replicas = -1
		}

		computedObjDetail, err := processPodSpec(podTemplSpec, inputManifestObj.Name, inputManifestObj.Kind, replicas)
		return tmpChkObjKind.Kind, computedObjDetail, err

	case "Deployment":
		var inputManifestObj appsv1.Deployment = appsv1.Deployment{}
		if err := yaml.Unmarshal(yamlRawdata, &inputManifestObj); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return tmpChkObjKind.Kind, nil, err
		}

		var podTemplSpec v1.PodSpec = inputManifestObj.Spec.Template.Spec
		var replicas int32
		if inputManifestObj.Spec.Replicas != nil {
			replicas = *inputManifestObj.Spec.Replicas
		} else  {
			replicas = -1
		}

		computedObjDetail, err := processPodSpec(podTemplSpec, inputManifestObj.Name, inputManifestObj.Kind, replicas)
		/* TODO: below idea of blindly setting to -1 is a bad way to update min/max repicas coz:
		- a given obj's hpa could be parsed before that object (vice-versa can happen as well)
		  - so, if the hpa is parsed first, then we will need to add that object in the `computedFileResult` structure with just the name, kind, min/max replicas. Naturally the cpu, ram will be added when we parse the actual deployment/sts corresp. to that object
		  - if the dep/sts is parse before hpa, then setting min/max replicas to -1 is ok
		- so, to account for both cases, first cheeck if that obj is already parsed & added to `computedFileResult` datastructure (this logic should be added in the `processEachObj` function)
		- 
		*/
		computedObjDetail.MinReplicas = -1
		computedObjDetail.MaxReplicas = -1
		return tmpChkObjKind.Kind, computedObjDetail, err
	
	case "HorizontalPodAutoscaler":
		var inputManifestObj autoscaling.HorizontalPodAutoscaler = autoscaling.HorizontalPodAutoscaler{}
		if err := yaml.Unmarshal(yamlRawdata, &inputManifestObj); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return tmpChkObjKind.Kind, nil, err
		}

		var hpaSpec autoscaling.HorizontalPodAutoscalerSpec = inputManifestObj.Spec
		// var hpaTargetName, hpaTargetKind string

		if targetObj := computedFileResult.chkIfObjAdded(hpaSpec.ScaleTargetRef.Kind, hpaSpec.ScaleTargetRef.Name); targetObj != nil {
			if err, computedObjDetail := processHPASpec(hpaSpec, targetObj); err != nil {
				return tmpChkObjKind.Kind, nil, err
			} else {
				return tmpChkObjKind.Kind, computedObjDetail, nil
			}
		} else {
			if err, computedObjDetail := processHPASpec(hpaSpec, nil); err != nil {
				return tmpChkObjKind.Kind, nil, err
			} else {
				return tmpChkObjKind.Kind, computedObjDetail, nil
			}
		}


		
		
	default:
		fmt.Println("Neither of preexisting object kinds match. Object kind: ", tmpChkObjKind.Kind)
		return tmpChkObjKind.Kind, nil, nil
	}

}



func processHPASpec(hpaSpec autoscaling.HorizontalPodAutoscalerSpec, computedObj *ObjDetail) (error, *ObjDetail) {
	if computedObj == nil {
		computedObj = &ObjDetail{
			MinReplicas:  *hpaSpec.MinReplicas,
			MaxReplicas: hpaSpec.MaxReplicas,
			ObjName: hpaSpec.ScaleTargetRef.Name,
			ObjKind: hpaSpec.ScaleTargetRef.Kind,
		}
		return nil, computedObj
	}else {
		computedObj.MinReplicas = *hpaSpec.MinReplicas
		computedObj.MaxReplicas = hpaSpec.MaxReplicas
		return nil, computedObj
	}
}
func processPodSpec(podTemplSpec v1.PodSpec, objectName string, objectKind string, objReplicas int32, computedFileResult *AllObjDetail ) (*ObjDetail, error) {

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

	

	if existingObj := computedFileResult.chkIfObjAdded(objectKind, objectName); existingObj != nil {
		existingObj.ObjName = objectName
		existingObj.ObjKind = objectKind
		existingObj.CpuReq = fmt.Sprintf("%v", cpuReq.AsDec())
		existingObj.CpuLim = fmt.Sprintf("%v", cpuLim.AsDec())
		existingObj.MemReq = memReq.Value()
		existingObj.MemLim = memLim.Value()
		return existingObj, nil
	} else {
		currObjData := &ObjDetail{
			ObjName: objectName,
			ObjKind: objectKind,
			CpuReq:  fmt.Sprintf("%v", cpuReq.AsDec()),
			CpuLim:  fmt.Sprintf("%v", cpuLim.AsDec())
			MemReq:  memReq.Value(), // humanReadable("memory", memReq.Value()),
			MemLim:  memLim.Value(), // humanReadable("memory", memLim.Value()),
			Replicas:  objReplicas,
			MinReplicas: -1,
			MaxReplicas: -1,
		}
		return currObjData, nil

	}
	
}

func renderOutput(reportVerbosity int ,renderData *AllObjDetail) {

	// w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	// fmt.Fprintln(w, "Name\tKind\tCPU\tMem")

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	// t.SetStyle(table.StyleColoredBright)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = true
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

	switch reportVerbosity{
	case 0:
		t.AppendHeader(table.Row{"Kind", "Name", "CPU", "CPU", "Memory", "Memory"}, rowConfigAutoMerge)
		t.AppendHeader(table.Row{"", "", "Request", "Limit", "Request", "Limit"})
		for objkind, objList := range renderData.Objects {
			for _, obj := range objList {
				t.AppendRow(table.Row{objkind, obj.ObjName, obj.CpuReq, obj.CpuLim, humanReadable("memory", obj.MemReq), humanReadable("memory", obj.MemLim)})
			}
		}

	case 1:
		t.AppendHeader(table.Row{"Kind", "Name", "Replicas", "CPU", "CPU", "Memory", "Memory"}, rowConfigAutoMerge)
		t.AppendHeader(table.Row{"", "", "",  "Request", "Limit", "Request", "Limit"})
		for objkind, objList := range renderData.Objects {
			for _, obj := range objList {
				t.AppendRow(table.Row{objkind, obj.ObjName, printReplicas(obj), obj.CpuReq, obj.CpuLim, humanReadable("memory", obj.MemReq), humanReadable("memory", obj.MemLim)})
			}
		}
	}
	
	t.Render()
}

func printReplicas(obj ObjDetail) (string) {
	if obj.Replicas != -1 {
		return strconv.Itoa(int(obj.Replicas))
	} else {
		return "NA"
	}
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
