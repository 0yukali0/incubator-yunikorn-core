package fair

import (
	"container/heap"

	"github.com/apache/yunikorn-core/pkg/common/configs"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/apps"
	customutil "github.com/apache/yunikorn-core/pkg/custom/util"
	"github.com/apache/yunikorn-core/pkg/log"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
	"go.uber.org/zap"
)

type FairManager struct {
	tenants      *urm.UserResourceManager
	apps         map[string]*apps.AppsHeap
	waitToDelete map[string]bool
}

func (f *FairManager) GetTenants() *urm.UserResourceManager {
	return f.tenants
}

func NewFairManager() *FairManager {
	return &FairManager{
		tenants:      urm.NewURM(),
		apps:         make(map[string]*apps.AppsHeap, 0),
		waitToDelete: make(map[string]bool, 0),
	}
}

func (f *FairManager) ParseUsersInPartitionConfig(conf configs.PartitionConfig) {
	records := f.GetTenants()
	users := customutil.ParseUsersInPartitionConfig(conf)
	for user, _ := range users {
		records.AddUser(user)
	}
}

func (f *FairManager) ParseUserInApp(input *objects.Application) {
	appID, user, _ := customutil.ParseApp(input)
	f.GetTenants().AddUser(user)
	if _, ok := f.apps[user]; !ok {
		f.apps[user] = apps.NewAppsHeap()
	}

	h := f.apps[user]
	info := apps.NewAppInfo(appID, input.SubmissionTime)
	heap.Push(h, info)
	log.Logger().Info("Add application in fair manager", zap.String("user", user), zap.String("applicationID", appID), zap.Int("heap", h.Len()))
}

func (f *FairManager) NextAppToSchedule() (bool, string, string) {
	user := f.GetTenants().GetMinResourceUser(f.apps)
	h, ok := f.apps[user]
	if !ok {
		//log.Logger().Info("Non existed user apps", zap.String("user", user))
		f.apps[user] = apps.NewAppsHeap()
		return false, "", ""
	}

	if h.Len() == 0 {
		//log.Logger().Info("User does not has apps", zap.String("user", user))
		return false, "", ""
	}

	target := heap.Pop(h).(*apps.AppInfo)
	if _, exist := f.waitToDelete[target.ApplicationID]; exist {
		delete(f.waitToDelete, target.ApplicationID)
		if h.Len() > 0 {
			target = heap.Pop(h).(*apps.AppInfo)
			heap.Push(h, target)
		} else {
			return false, "", ""
		}
	} else {
		heap.Push(h, target)
	}

	appID := target.ApplicationID
	//log.Logger().Info("User has apps", zap.String("user", user), zap.String("appid", target.ApplicationID), zap.Int("heap", h.Len()))
	return true, user, appID
}

func (f *FairManager) UpdateScheduledApp(input *objects.Application) {
	appID, user, res := customutil.ParseApp(input)
	f.waitToDelete[appID] = true
	//log.Logger().Info("Update scheduled app", zap.String("app", appID), zap.String("user", user))
	if h, ok := f.apps[user]; !ok {
		log.Logger().Error("Non existed app update", zap.String("app", appID), zap.String("user", user))
	} else {
		log.Logger().Info("Update scheduled app", zap.Int("heap", h.Len()))
		bk := make([]*apps.AppInfo, 0)
		for h.Len() > 0 || len(f.waitToDelete) > 0 {
			target := heap.Pop(h).(*apps.AppInfo)
			id := target.ApplicationID
			if _, exist := f.waitToDelete[id]; !exist {
				log.Logger().Info("Delete app is not in the heap", zap.String("appid", id))
				bk = append(bk, target)
			} else {
				delete(f.waitToDelete, id)
				log.Logger().Info("Delete app", zap.String("appid", id), zap.Int("heap", h.Len()))
			}
		}

		for _, element := range bk {
			heap.Push(h, element)
		}
	}
	f.GetTenants().UpdateUser(user, res)
}
