package estimate

// This datastructure collects all the data
// related to a single input file passed to the tool
type AllObjDetail struct {
	Objects             map[string][]*ObjDetail
	GrossTotalResources [4][3]float32
}

func (a *AllObjDetail) chkIfObjAdded(targetObjKind string, targetObjName string) *ObjDetail {

	objects, exists := a.Objects[targetObjKind]
	if exists {
		for _, computedObj := range objects {
			if computedObj.ObjName == targetObjName {
				return computedObj
			}
		}
		return nil
	} else {
		return nil
	}
}

func (a *AllObjDetail) computeGrossTotalResources() {

	// TODO(v2): 4 nested loops... uggg.... see if this can be optimised later without too much rabbithole
	for _, k8sobjList := range a.Objects {
		for _, obj := range k8sobjList {
			for i, resourceType := range obj.TotalResourceForWholeObj {
				for j, _ := range resourceType {
					if obj.TotalResourceForWholeObj[i][j] > 0 {
						a.GrossTotalResources[i][j] += obj.TotalResourceForWholeObj[i][j]
					}
				}
			}
		}
	}

	// fmt.Println("Printing GrossTotalResources: ", a.GrossTotalResources)
}

/////////////////////////////////////////////////////

// This datastructure helps abstract
// all the details related to a single K8s Object
// (takes away the pain of passing multiple fields like cpu,mem, replicas etc.) 
type ObjDetail struct {
	ObjKind                  string
	ObjName                  string
	MemReq                   float32 // TODO: Context is change all of this to float64 such that you can do on-the-fly computation for various values
	MemLim                   float32
	CpuReq                   float32
	CpuLim                   float32
	Replicas                 int32
	MinReplicas              int32
	MaxReplicas              int32
	HPAPresent               bool
	TotalResourceForWholeObj [4][3]float32 // Schema: [ [ rep, min, max for cpuReq ] [ rep, min, max for cpuLim ] [ rep, min, max for memReq ] [ rep, min, max for memLim ]  ]

}

// Ik this is a hack & i will have to refactor my datatypes to make the whole thing generalisable
// (especially when I start dealing with Volumes/Disks coz 1 pod can have multiple disks).
// But that's a problem for future me and right now, I am priotising shipping of v1
func (obj *ObjDetail) computeTotals() {

	/*
		Schema: [ [ rep, min, max for cpuReq ] [ rep, min, max for cpuLim ] [ rep, min, max for memReq ] [ rep, min, max for memLim ]  ]
	*/

	obj.TotalResourceForWholeObj[0][0] = float32(obj.Replicas) * obj.CpuReq
	obj.TotalResourceForWholeObj[1][0] = float32(obj.Replicas) * obj.CpuLim
	obj.TotalResourceForWholeObj[2][0] = float32(obj.Replicas) * obj.MemReq
	obj.TotalResourceForWholeObj[3][0] = float32(obj.Replicas) * obj.MemLim

	obj.TotalResourceForWholeObj[0][1] = float32(obj.MinReplicas) * obj.CpuReq
	obj.TotalResourceForWholeObj[1][1] = float32(obj.MinReplicas) * obj.CpuLim
	obj.TotalResourceForWholeObj[2][1] = float32(obj.MinReplicas) * obj.MemReq
	obj.TotalResourceForWholeObj[3][1] = float32(obj.MinReplicas) * obj.MemLim

	obj.TotalResourceForWholeObj[0][2] = float32(obj.MaxReplicas) * obj.CpuReq
	obj.TotalResourceForWholeObj[1][2] = float32(obj.MaxReplicas) * obj.CpuLim
	obj.TotalResourceForWholeObj[2][2] = float32(obj.MaxReplicas) * obj.MemReq
	obj.TotalResourceForWholeObj[3][2] = float32(obj.MaxReplicas) * obj.MemLim

}
