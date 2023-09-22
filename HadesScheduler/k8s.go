package main

import "github.com/Mtze/HadesCI/shared/payload"

type JobScheduler interface {
	ScheduleJob(job payload.BuildJob) error
}

type K8sScheduler struct{}

func (k *K8sScheduler) ScheduleJob(job payload.BuildJob) error {
	return nil
}
