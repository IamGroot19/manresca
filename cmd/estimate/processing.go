package estimate

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	table "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	yaml "sigs.k8s.io/yaml"

	"k8s.io/kubernetes/pkg/apis/autoscaling"
	// "github.com/spf13/cobra"
)

// This datastructure collects all the data
//  related to a single input file passed to the tool
type AllObjDetail struct {
	Objects map[string][]*ObjDetail
}

func (a *AllObjDetail) chkIfObjAdded(targetObjKind string, targetObjName string) (*ObjDetail) {

	objects, exists :=  a.Objects[targetObjKind]
	if exists {
		for _, computedObj := range objects  {
			if computedObj.ObjName == targetObjName {
				return computedObj
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
	computedFileResult.Objects = make(map[string][]*ObjDetail)
	// var computedObjKind string

	for _, manifest := range manifests {
		processEachObject([]byte(manifest), computedFileResult)
		// fmt.Println("computedFileResult: ", *&computedFileResult.Objects, "\n")
		
	}
	renderOutput(reportVerbosity, computedFileResult) // print tabular summary
	

}

// This method processes each k8s object and
//  stores the resulting data in an object of
// the `ObjDetail` type, which is appended to bigger type  `AllObjDetail` 
// It outsources actual resources extraction to a separate fcuntion
// since podspec is common to both deployments and statefulsets 
func processEachObject(yamlRawdata []byte, computedFileResult *AllObjDetail) {

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
			return
		}

		var podTemplSpec v1.PodSpec = inputManifestObj.Spec.Template.Spec
		var replicas int32
		if inputManifestObj.Spec.Replicas != nil {
			replicas = *inputManifestObj.Spec.Replicas
		} else  {
			replicas = -1
		}

		if  err := processPodSpec(podTemplSpec, inputManifestObj.Name, inputManifestObj.Kind, replicas, computedFileResult); err != nil {
			fmt.Printf("Error processing PodSpec for an Objectc Kind %s: %v\n", tmpChkObjKind.Kind, err)
			return
		} else {
			return
		}
		
	case "Deployment":
		var inputManifestObj appsv1.Deployment = appsv1.Deployment{}
		if err := yaml.Unmarshal(yamlRawdata, &inputManifestObj); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return
		}

		var podTemplSpec v1.PodSpec = inputManifestObj.Spec.Template.Spec
		var replicas int32
		if inputManifestObj.Spec.Replicas != nil {
			replicas = *inputManifestObj.Spec.Replicas
		} else  {
			replicas = -1
		}

		if err := processPodSpec(podTemplSpec, inputManifestObj.Name, inputManifestObj.Kind, replicas, computedFileResult); err != nil {
			fmt.Printf("Error processing PodSpec for an Objectc Kind %s: %v\n", tmpChkObjKind.Kind, err)
			return
		} else {
			return
		}

	case "HorizontalPodAutoscaler":
		var inputManifestObj autoscaling.HorizontalPodAutoscaler = autoscaling.HorizontalPodAutoscaler{}
		if err := yaml.Unmarshal(yamlRawdata, &inputManifestObj); err != nil {
			fmt.Println("Error unmarshalling yaml data into deployment struct type: ", err)
			return 
		}

		var hpaSpec autoscaling.HorizontalPodAutoscalerSpec = inputManifestObj.Spec
		if err := processHPASpec(hpaSpec, computedFileResult); err != nil {
			fmt.Printf("Unable to process Spec for the HPA object %s due to Error: %v\n", inputManifestObj.Name, err)
			return 		
		} else {
			return 
		}
		
	default:
		// fmt.Println("Neither of preexisting object kinds match. Object kind: ", tmpChkObjKind.Kind)
		return
	}

}



func processHPASpec(hpaSpec autoscaling.HorizontalPodAutoscalerSpec, computedFileResult *AllObjDetail) (error) {
 
	computedObj := computedFileResult.chkIfObjAdded(hpaSpec.ScaleTargetRef.Kind, hpaSpec.ScaleTargetRef.Name)


	if computedObj == nil {
		// fmt.Println("computedObj is nil: ", *computedObj)
		computedObj = &ObjDetail{
			MinReplicas:  *hpaSpec.MinReplicas,
			MaxReplicas: hpaSpec.MaxReplicas,
			ObjName: hpaSpec.ScaleTargetRef.Name,
			ObjKind: hpaSpec.ScaleTargetRef.Kind,
		}
		(*computedFileResult).Objects[hpaSpec.ScaleTargetRef.Kind] = append((*computedFileResult).Objects[hpaSpec.ScaleTargetRef.Kind], computedObj)
		return nil
	}else {
		// fmt.Println("compuedObj already exists (so will just edit up min/max replicas): ", *computedObj)
		computedObj.MinReplicas = *hpaSpec.MinReplicas
		computedObj.MaxReplicas = hpaSpec.MaxReplicas
		return  nil
	}
}
func processPodSpec(podTemplSpec v1.PodSpec, objectName string, objectKind string, objReplicas int32, computedFileResult *AllObjDetail ) (error) {

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
		// fmt.Printf("Obj %s already added, so just editing it to add hpa min/max repica count", existingObj.ObjName)
		existingObj.ObjName = objectName
		existingObj.ObjKind = objectKind
		existingObj.CpuReq = fmt.Sprintf("%v", cpuReq.AsDec())
		existingObj.CpuLim = fmt.Sprintf("%v", cpuLim.AsDec())
		existingObj.MemReq = memReq.Value()
		existingObj.MemLim = memLim.Value()
		return nil
	} else {
		computedObj := &ObjDetail{
			ObjName: objectName,
			ObjKind: objectKind,
			CpuReq:  fmt.Sprintf("%v", cpuReq.AsDec()),
			CpuLim:  fmt.Sprintf("%v", cpuLim.AsDec()),
			MemReq:  memReq.Value(), // humanReadable("memory", memReq.Value()),
			MemLim:  memLim.Value(), // humanReadable("memory", memLim.Value()),
			Replicas:  objReplicas,
			MinReplicas: -1,
			MaxReplicas: -1,
		}
		(*computedFileResult).Objects[computedObj.ObjKind] = append((*computedFileResult).Objects[computedObj.ObjKind], computedObj)
		return nil

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
	// t.Style().Box.PaddingLeft = ""
	t.Style().Box.PaddingRight = "  "

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
		t.AppendHeader(table.Row{"", "", "(Replicas / HPA Min / HPA Max)",  "Request", "Limit", "Request", "Limit"})
		for objkind, objList := range renderData.Objects {
			for _, obj := range objList {
				t.AppendRow(table.Row{objkind, obj.ObjName, printReplicas(obj), obj.CpuReq, obj.CpuLim, humanReadable("memory", obj.MemReq), humanReadable("memory", obj.MemLim)})
			}
		}
	

	case 2:
		fmt.Println("Add logic for case2: multiply cpu/mem nos. with replicas")
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true, AlignHeader: text.AlignCenter, Align: text.AlignLeft},
		{Number: 2, AutoMerge: true, AlignHeader: text.AlignCenter, Align: text.AlignLeft},
		{Number: 3, AutoMerge: true, AlignHeader: text.AlignCenter, Align: text.AlignCenter},
		{Number: 4, AutoMerge: true, AlignHeader: text.AlignLeft, Align: text.AlignLeft},
		{Number: 5, AutoMerge: true, AlignHeader: text.AlignLeft, Align: text.AlignLeft},
		{Number: 6, AutoMerge: true, AlignHeader: text.AlignLeft, Align: text.AlignLeft},
		{Number: 7, AutoMerge: true, AlignHeader: text.AlignLeft, Align: text.AlignLeft},
	})
	t.Render()
}

func printReplicas(obj *ObjDetail) (string) {
	var replicaString strings.Builder

	if obj.Replicas != -1 {
		replicaString.WriteString( strconv.Itoa(int(obj.Replicas)) )
	} else {
		replicaString.WriteString("x")	
	}
	
	if obj.MinReplicas != -1 {
		replicaString.WriteString(" / ")
		replicaString.WriteString( strconv.Itoa(int(obj.MinReplicas)) )
		
	} else {
		replicaString.WriteString(" / x")
	}

	if  obj.MaxReplicas != -1 {
		replicaString.WriteString(" / ")
		replicaString.WriteString( strconv.Itoa(int(obj.MaxReplicas)) )
	}else {
		replicaString.WriteString(" / x")
	}

	return replicaString.String()
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
		return strconv.FormatFloat(finalQty, 'f', 3, 64) + " " + units[ct]

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
