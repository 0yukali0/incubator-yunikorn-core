package urm

import (
	"container/heap"
	"errors"

	"github.com/apache/yunikorn-core/pkg/common/resources"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/apps"
	"github.com/apache/yunikorn-core/pkg/custom/fair/urm/users"
	"github.com/apache/yunikorn-core/pkg/log"
	"go.uber.org/zap"
)

type UserResourceManager struct {
	existedUser map[string]*userApps
	priority    *users.UsersHeap
}

func NewURM() *UserResourceManager {
	return &UserResourceManager{
		existedUser: make(map[string]*userApps, 0),
		priority:    users.NewUserHeap(),
	}
}

func (u *UserResourceManager) AddUser(name string) {
	if _, ok := u.existedUser[name]; !ok {
		u.existedUser[name] = NewUserApps()
	}
}

func (u *UserResourceManager) GetMinResourceUser(apps map[string]*apps.AppsHeap, clusterResource *resources.Resource) string {
	clusterRes := clusterResource.Clone()
	for userName, apps := range u.existedUser {
		heap.Push(u.priority, users.NewScore(userName, apps.ComputeGlobalDominantResource(clusterRes)))
	}

	if u.priority.Len() == 0 {
		log.Logger().Warn("userheap should not be empty when getting min")
		return ""
	}

	// return the user with min resource if this user has unscheduled apps
	var s *users.Score
	for u.priority.Len() > 0 {
		tmp := heap.Pop(u.priority).(*users.Score)
		if requests, ok := apps[tmp.GetUser()]; ok {
			if requests.Len() > 0 {
				s = tmp
				break
			}
		}
	}

	if s == nil {
		return ""
	}
	return s.GetUser()
}

func (u *UserResourceManager) Allocate(user string, appID string, res *resources.Resource) {
	u.existedUser.RunApp(appID, res)
}

func (u *UserResourceManager) Release(user string, appID string) {
	if apps, ok := u.existedUser[user]; ok {
		apps.CompeleteApp(appID)
	}
}
