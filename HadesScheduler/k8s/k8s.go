package k8s

import (
	"context"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct{}

type K8sConfig struct{}

func NewK8sScheduler() Scheduler {
	log.Debug("Initializing Kubernetes scheduler")

	var k8sCfg K8sConfig
	utils.LoadConfig(&k8sCfg)
	log.Debugf("Kubernetes config: %+v", k8sCfg)

	return Scheduler{}
}

func (k Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	log.Debug("Scheduling job in Kubernetes")
	return nil
}
