package estimate

import (
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
//
//	stores the resulting data in an object of
//
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
		} else {
			replicas = -1
		}

		if err := processPodSpec(podTemplSpec, inputManifestObj.Name, inputManifestObj.Kind, replicas, computedFileResult); err != nil {
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
		} else {
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

func processHPASpec(hpaSpec autoscaling.HorizontalPodAutoscalerSpec, computedFileResult *AllObjDetail) error {

	computedObj := computedFileResult.chkIfObjAdded(hpaSpec.ScaleTargetRef.Kind, hpaSpec.ScaleTargetRef.Name)

	if computedObj == nil {
		// fmt.Println("computedObj is nil: ", *computedObj)
		computedObj = &ObjDetail{
			MinReplicas: *hpaSpec.MinReplicas,
			MaxReplicas: hpaSpec.MaxReplicas,
			ObjName:     hpaSpec.ScaleTargetRef.Name,
			ObjKind:     hpaSpec.ScaleTargetRef.Kind,
			CpuReq:      0.0,
			CpuLim:      0.0,
			MemReq:      0.0,
			MemLim:      0.0,
		}
		(*computedFileResult).Objects[hpaSpec.ScaleTargetRef.Kind] = append((*computedFileResult).Objects[hpaSpec.ScaleTargetRef.Kind], computedObj)
		return nil
	} else {
		// fmt.Println("compuedObj already exists (so will just edit up min/max replicas): ", *computedObj)
		computedObj.MinReplicas = *hpaSpec.MinReplicas
		computedObj.MaxReplicas = hpaSpec.MaxReplicas
		return nil
	}
}
func processPodSpec(podTemplSpec v1.PodSpec, objectName string, objectKind string, objReplicas int32, computedFileResult *AllObjDetail) error {

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

	// k8s.io/apimachinery/pkg/api/resource
	if existingObj := computedFileResult.chkIfObjAdded(objectKind, objectName); existingObj != nil {
		// fmt.Printf("Obj %s already added, so just editing it to add hpa min/max repica count", existingObj.ObjName)
		existingObj.ObjName = objectName
		existingObj.ObjKind = objectKind

		// though the function says "ApproximateFloat64",
		// the approximation is not that relevant here because:
		// - your typical CPU is at best going to vary from 0.001 aka 1m
		// 		(anything smaller is meaningless & even kubernetes rounds it off to 1m)
		// - your typical RAM is going to vary from few MBs to Terabytes (maybe PetaByte at worst?)
		// For both the resources, even float16 will suffice. I'm picking float32 just as a precaution.
		existingObj.CpuReq = float32(cpuReq.AsApproximateFloat64())
		existingObj.CpuLim = float32(cpuLim.AsApproximateFloat64())
		existingObj.MemReq = float32(memReq.Value())
		existingObj.MemLim = float32(memLim.Value())
		return nil
	} else {
		computedObj := &ObjDetail{
			ObjName: objectName,
			ObjKind: objectKind,

			CpuReq: float32(cpuReq.AsApproximateFloat64()),
			CpuLim: float32(cpuLim.AsApproximateFloat64()),

			MemReq: float32(memReq.Value()), // humanReadable("memory", memReq.Value()),
			MemLim: float32(memLim.Value()), // humanReadable("memory", memLim.Value()),

			Replicas:    objReplicas,
			MinReplicas: -1,
			MaxReplicas: -1,
		}
		(*computedFileResult).Objects[computedObj.ObjKind] = append((*computedFileResult).Objects[computedObj.ObjKind], computedObj)
		return nil

	}

}

func renderOutput(reportVerbosity int, renderData *AllObjDetail) {

	// w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	// fmt.Fprintln(w, "Name\tKind\tCPU\tMem")

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	// t.SetStyle(table.StyleColoredBright)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = true
	// t.Style().Box.PaddingLeft = ""
	t.Style().Box.PaddingRight = "  "

	// rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

	switch reportVerbosity {
	// case 0:
	// 	t.AppendHeader(table.Row{"Kind", "Name", "CPU", "CPU", "Memory", "Memory"}, rowConfigAutoMerge)
	// 	t.AppendHeader(table.Row{"", "", "Request", "Limit", "Request", "Limit"})
	// 	for objkind, objList := range renderData.Objects {
	// 		for _, obj := range objList {
	// 			t.AppendRow(table.Row{objkind, obj.ObjName, obj.CpuReq, obj.CpuLim, humanReadable("memory", obj.MemReq), humanReadable("memory", obj.MemLim)})
	// 		}
	// 	}

	case 0:
		fmt.Println("Summary: Prints Replica Counts and Resource Usage at a per Object level (doesnt multiply resources by replica count nor does it show total resource usage)\nIf a certain value is not provided (or is zero), then an underscore is printed as a placeholder")
		t.AppendHeader(table.Row{"Kind", "Name", "Replicas", "CPU", "CPU", "Memory", "Memory"}, table.RowConfig{AutoMerge: true})
		t.AppendHeader(table.Row{"", "", "(Replicas / HPA Min / HPA Max)", "Request", "Limit", "Request", "Limit"})
		for objkind, objList := range renderData.Objects {
			for _, obj := range objList {
				t.AppendRow(table.Row{objkind, obj.ObjName, printReplicas(obj), humanReadable("cpu", obj.CpuReq), humanReadable("cpu", obj.CpuLim), humanReadable("mem", obj.MemReq), humanReadable("mem", obj.MemLim)})
			}
		}

	case 1:
		fmt.Printf("Summary: Print repiica count, total resources per object (i.e per pod resources multiplied by replica coun) and Net Total resource required by whole chart.\n         But in this case, given that HPAs are also involved, the Resource Columns for each object would show  3 numbers accounting (Replicas, HPAMin, HPAMax).\nIf a certain value is not provided (or is zero), then an underscore is printed as a placeholder")
		t.AppendHeader(table.Row{"Kind", "Name", "Replicas", "CPU", "CPU", "Memory", "Memory"}, table.RowConfig{AutoMerge: true})
		t.AppendHeader(table.Row{"", "", "(Replicas / HPA Min / HPA Max)", "Request (Replicas / Min / Max)", "Limit (Replicas / Min / Max)", "Request (Replicas / Min / Max)", "Limit (Replicas / Min / Max)"})

		for objkind, objList := range renderData.Objects {
			for _, obj := range objList {
				obj.computeTotals()
				t.AppendRow(table.Row{objkind, obj.ObjName, printReplicas(obj), printTotals("cpu", obj.TotalResourceForWholeObj[0]), printTotals("cpu", obj.TotalResourceForWholeObj[1]), printTotals("mem", obj.TotalResourceForWholeObj[2]), printTotals("mem", obj.TotalResourceForWholeObj[3])})
			}
		}
		renderData.computeGrossTotalResources()
		t.AppendFooter(table.Row{"", "", "Total", printTotals("cpu", renderData.GrossTotalResources[0]), printTotals("cpu", renderData.GrossTotalResources[1]), printTotals("mem", renderData.GrossTotalResources[2]), printTotals("mem", renderData.GrossTotalResources[3])})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true, AlignHeader: text.AlignCenter, Align: text.AlignLeft, AlignFooter: text.AlignLeft},
		{Number: 2, AutoMerge: true, AlignHeader: text.AlignCenter, Align: text.AlignLeft, AlignFooter: text.AlignLeft},
		{Number: 3, AutoMerge: false, AlignHeader: text.AlignCenter, Align: text.AlignCenter, AlignFooter: text.AlignLeft},
		{Number: 4, AutoMerge: false, AlignHeader: text.AlignCenter, Align: text.AlignCenter, AlignFooter: text.AlignCenter},
		{Number: 5, AutoMerge: false, AlignHeader: text.AlignCenter, Align: text.AlignCenter, AlignFooter: text.AlignCenter},
		{Number: 6, AutoMerge: false, AlignHeader: text.AlignCenter, Align: text.AlignCenter, AlignFooter: text.AlignCenter},
		{Number: 7, AutoMerge: false, AlignHeader: text.AlignCenter, Align: text.AlignCenter, AlignFooter: text.AlignCenter},
	})
	t.Render()
}

// For the verbosity flag value of `1`
func printReplicas(obj *ObjDetail) string {
	var replicaString strings.Builder

	if obj.Replicas != -1 {
		replicaString.WriteString(strconv.Itoa(int(obj.Replicas)))
	} else {
		replicaString.WriteString("_\t/\t")
	}

	if obj.MinReplicas != -1 {
		replicaString.WriteString(strconv.Itoa(int(obj.MinReplicas)))

	} else {
		replicaString.WriteString("\t/\t_")
	}

	if obj.MaxReplicas != -1 {
		replicaString.WriteString(strconv.Itoa(int(obj.MaxReplicas)))
	} else {
		replicaString.WriteString("\t/\t_")
	}

	return replicaString.String()
}

// For the verbosity flag value `2`.
// This method combines totals for replicas / HPAmin / HPAmax
// together into a single string
func printTotals(qtyType string, repMinMax [3]float32) string {
	var result strings.Builder

	switch qtyType {
	case "cpu":
		for i := range repMinMax {
			if repMinMax[i] < 0 {
				result.WriteString("_\t/\t")
			} else {
				result.WriteString(humanReadable("cpu", repMinMax[i]) + "\t/\t")
			}
		}
	case "mem":
		for i := range repMinMax {
			if repMinMax[i] < 0 {
				result.WriteString("_\t/\t")
			} else {
				result.WriteString(humanReadable("mem", repMinMax[i]) + "\t/\t")
			}
		}
	default:
		result.WriteString("Something went wrong")
	}
	return strings.TrimSuffix(strings.TrimSpace(result.String()), "/")
}

// receives floats in byes and returns it as a human readable string
func humanReadable(qtyType string, size float32) string {
	switch qtyType {
	case "cpu":
		if size == -0.0 || size == 0.0 {
			return "_\t"
		} else {
			return strconv.FormatFloat(float64(size), 'f', 1, 64)
		}

	case "mem":
		if size == 0.0 || size == -0.0 {
			return "_\t"
		} else {
			units := []string{"B", "Ki", "Mi", "Gi", "Ti", "Pi"}
			var finalQty float64 = float64(size)
			var ct int8 = 0
			for finalQty/1024 > 1 {
				ct += 1
				finalQty = finalQty / 1024
			}
			return strconv.FormatFloat(finalQty, 'f', 1, 64) + " " + units[ct]
		}
	}
	return ""
}
