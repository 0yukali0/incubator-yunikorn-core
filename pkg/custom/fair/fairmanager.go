package fair

import (
	"container/heap"

	"github.com/apache/yunikorn-core/pkg/common/configs"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/apps"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/users"
	customutil "github.com/apache/yunikorn-core/pkg/custom/util"
	"github.com/apache/yunikorn-core/pkg/log"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
	"go.uber.org/zap"
)

type FairManager struct {
	tenants *urm.UserResourceManager
	apps    map[string]*apps.AppsHeap
}

func (f *FairManager) GetTenants() *urm.UserResourceManager {
	return f.tenants
}

func NewFairManager() *FairManager {
	return &FairManager{
		tenants: urm.NewURM(),
		apps:    make(map[string]*apps.AppsHeap, 0),
	}
}

func (f *FairManager) ContinueSchedule() bool {
	minUser := f.GetTenants().GetMinResourceUser()
	if h, ok := f.apps[minUser]; !ok {
		f.apps[minUser] = apps.NewAppsHeap()
		return false
	} else {
		if h.Len() == 0 {
			return false
		}
	}
	return true
}

func (f *FairManager) ParseUsersInPartitionConfig(conf configs.PartitionConfig) {
	records := f.GetTenants()
	users := customutil.ParseUsersInPartitionConfig(conf)
	for user, _ := range users {
		records.AddUser(user)
	}
}

func (f *FairManager) ParseUserInApp(app *objects.Application) {
	user := app.GetUser().User
	f.GetTenants().AddUser(user)
	log.Logger().Info("Application user", zap.String("user", user))
	if _, ok := f.apps[user]; !ok {
		f.apps[user] = apps.NewAppsHeap()
	}

	log.Logger().Info("Add application in fair manager", zap.String("applicationID", app.ApplicationID))
	heap.Push(f.apps[user], apps.NewAppInfo(app.ApplicationID, app.SubmissionTime))
}

func (f *FairManager) NextAppToSchedule() (bool, string, string) {
	user := f.GetTenants().GetMinResourceUser()
	h, ok := f.apps[user]
	if !ok {
		f.apps[user] = apps.NewAppsHeap()
		return false, "", ""
	}

	if h.Len() == 0 {
		return false, "", ""
	}

	target := heap.Pop(h).(*apps.AppInfo)
	heap.Push(h, target)
	return true, user, target.ApplicationID
}

func (f *FairManager) UpdateScheduledApp(user string, resources map[string]int64, duration uint64) {
	if h, ok := f.apps[user]; !ok {
		log.Logger().Error("Non existed app update", zap.String("app", user))
	} else {
		heap.Pop(h)
	}
	resources["Duration"] = int64(duration)
	f.GetTenants().UpdateUser(users.NewScoreInfo(user, resources))
}
