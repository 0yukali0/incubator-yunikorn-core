package fair

import (
	"container/heap"

	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/apps"
)

func (f *FairManager) NextAppToSchedule() (bool, string, string) {
	user := f.GetTenants().GetMinResourceUser(f.unscheduledApps, f.clusterResource)
	h, ok := f.unscheduledApps[user]
	if !ok {
		f.unscheduledApps[user] = apps.NewAppsHeap()
		return false, "", ""
	}

	if h.Len() == 0 {
		return false, "", ""
	}

	target := heap.Pop(h).(*apps.AppInfo)
	if _, exist := f.scheduledApps[target.ApplicationID]; exist {
		delete(f.scheduledApps, target.ApplicationID)
		if h.Len() > 0 {
			target = heap.Pop(h).(*apps.AppInfo)
			heap.Push(h, target)
		} else {
			return false, "", ""
		}
	} else {
		heap.Push(h, target)
	}

	return true, user, target.ApplicationID
}