package monitor

import (
	"fmt"
	"github.com/apache/yunikorn-core/pkg/common/resources"
	"github.com/apache/yunikorn-core/pkg/custom/util"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
	sicommon "github.com/apache/yunikorn-scheduler-interface/lib/go/common"
	"sort"
	"time"

	excel "github.com/xuri/excelize/v2"
)

type NodeUtilizationMonitor struct {
	nodes             map[string]*NodeResource
	id                map[string]string
	GlobalEventUnique map[uint64]bool
	GlobalEvent       []uint64
	startTime         time.Time
	file              *excel.File
}

func NewUtilizationMonitor() *NodeUtilizationMonitor {
	file := excel.NewFile()
	file.NewSheet(migsheet)
	return &NodeUtilizationMonitor{
		nodes:             make(map[string]*NodeResource),
		GlobalEventUnique: make(map[uint64]bool),
		GlobalEvent:       make([]uint64, 0),
		id:                make(map[string]string),
		file:              file,
	}
}

func (m *NodeUtilizationMonitor) SetStartTime(t time.Time) {
	m.startTime = t
}

func (m *NodeUtilizationMonitor) Allocate(nodeID string, allocatedTime time.Time, req *resources.Resource) {
	if n, ok := m.nodes[nodeID]; ok {
		releaseTime := allocatedTime.Add(time.Second * time.Duration(req.Resources[sicommon.Duration]))
		d1 := SubTimeAndTranslateToUint64(allocatedTime, m.startTime)
		m.AddGlobalEventsTime(d1)
		d2 := SubTimeAndTranslateToUint64(releaseTime, m.startTime)
		m.AddGlobalEventsTime(d2)
		n.Allocate(d1, d2, req)
	}
}

func (m *NodeUtilizationMonitor) AddNode(n *objects.Node) {
	nodeID, avial, cap := util.ParseNode(n)
	if _, ok := m.nodes[nodeID]; !ok {
		m.id[nodeID] = excelColForUtilization[len(m.nodes)]
		m.nodes[nodeID] = NewNodeResource(avial, cap)
		idLetter := m.id[nodeID]
		m.file.SetCellValue(migsheet, fmt.Sprintf("%s%d", idLetter, 1), nodeID)
	}
}

func (m *NodeUtilizationMonitor) AddGlobalEventsTime(t uint64) {
	if _, ok := m.GlobalEventUnique[t]; !ok {
		m.GlobalEventUnique[t] = true
		m.GlobalEvent = append(m.GlobalEvent, t)
	}
}

func (m *NodeUtilizationMonitor) Save() {
	DeleteExistedFile(utilizationfiltpath)
	sort.Slice(m.GlobalEvent, func(i, j int) bool { return m.GlobalEvent[i] < m.GlobalEvent[j] })
	for index, timestamp := range m.GlobalEvent {
		placeNum := uint64(index + 2)
		m.file.SetCellValue(fairness, fmt.Sprintf("%s%d", timestampLetterOfUitlization, placeNum), timestamp)
		nodesRes := make([]*resources.Resource, 0)
		for nodeID, nodeRes := range m.nodes {
			_ = nodeRes.AllocateResource(timestamp)
			utilization := resources.CalculateAbsUsedCapacity(nodeRes.cap, resources.Sub(nodeRes.cap, nodeRes.avaialble))
			nodesRes = append(nodesRes, utilization.Clone())
			// mig
			idLetter := m.id[nodeID]
			m.file.SetCellValue(migsheet, fmt.Sprintf("%s%d", idLetter, placeNum), int64(resources.MIG(utilization)))
		}
		average := resources.Average(nodesRes)
		gapSum := resources.NewResource()
		// sum += (utilization - average utilization)^2
		for _, n := range nodesRes {
			gap := resources.Sub(n, average)
			powerGap := resources.Power(gap, float64(2))
			gapSum = resources.Add(gapSum, powerGap)
		}
		// Max deviation = Max(SQRT(sum including cpu and memory))
		gapSum = resources.Power(gapSum, float64(0.5))
		standardDeviation := resources.Max(gapSum)
		m.file.SetCellValue(migsheet, fmt.Sprintf("%s%d", bias, placeNum), int64(standardDeviation))
	}
}

type NodeResource struct {
	avaialble, cap *resources.Resource
	events         map[uint64]*resources.Resource
}

func NewNodeResource(avaialble, cap *resources.Resource) *NodeResource {
	return &NodeResource{
		avaialble: avaialble.Clone(),
		cap:       cap.Clone(),
		events:    make(map[uint64]*resources.Resource),
	}
}

func (n *NodeResource) Allocate(allocated, release uint64, req *resources.Resource) {
	if _, ok := n.events[allocated]; !ok {
		n.events[allocated] = resources.Sub(nil, req.Clone())
	} else {
		n.events[allocated] = resources.Sub(n.events[allocated], req.Clone())
	}

	if _, ok := n.events[release]; !ok {
		n.events[release] = req.Clone()
	} else {
		n.events[release] = resources.Add(n.events[allocated], req.Clone())
	}
	return
}

func (n *NodeResource) AllocateResource(timestamp uint64) bool {
	if value, ok := n.events[timestamp]; !ok {
		return false
	} else {
		n.avaialble = resources.Add(n.avaialble, value)
	}
	return true
}
