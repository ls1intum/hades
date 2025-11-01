package controller

import (
	"context"
	"fmt"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *BuildJobReconciler) resolvePodName(ctx context.Context, bj *buildv1.BuildJob) (string, error) {

	cli := r.K8sClient.CoreV1().Pods(bj.Namespace)

	// if we have a pod name, return
	if p, err := cli.Get(ctx, bj.Name, metav1.GetOptions{}); err == nil {
		return p.Name, nil
	}

	// else: find the pod name by looking at the job name
	jobName := fmt.Sprintf("buildjob-%s", bj.Name)
	if lst, err := cli.List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	}); err == nil {
		if len(lst.Items) == 1 {
			return lst.Items[0].Name, nil
		}
		if len(lst.Items) > 1 {
			return "", fmt.Errorf("found multiple pods with label job-name=%s; expected exactly 1", jobName)
		}
	}

	return "", fmt.Errorf("pod for jobID %s not found yet", bj.Name)
}

// func (r *BuildJobReconciler) readAndPublishLogs(...) error { }
// func (r *BuildJobReconciler) publishLogs(...) error { }
