/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/utils"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"
	"github.com/ls1intum/hades/HadesScheduler/HadesOperator/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

type NSConfig struct {
	WatchNamespace string `env:"WATCH_NAMESPACE"`
}

type NCConfig struct {
	NatsConfig utils.NatsConfig
}

type OperatorConfig struct {
	DeleteOnComplete string `env:"DELETE_ON_COMPLETE" envDefault:"true"`
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(buildv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var enableLeaderElection bool
	var probeAddr string
	var enableDevMode bool

	if os.Getenv("DEV_MODE") == "true" {
		enableDevMode = true
	}

	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8083", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	opts := zap.Options{Development: enableDevMode}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var nsConfig NSConfig
	utils.LoadConfig(&nsConfig)

	var operatorConfig OperatorConfig
	utils.LoadConfig(&operatorConfig)

	delOnComplete, _ := strconv.ParseBool(operatorConfig.DeleteOnComplete)

	if nsConfig.WatchNamespace != "" {
		setupLog.Info("scoping cache to a single namespace", "namespace", nsConfig.WatchNamespace)
	} else {
		setupLog.Info("no WATCH_NAMESPACE set; manager will watch cluster-wide")
	}

	mgrOpts := ctrl.Options{
		Scheme: scheme,
		// Disable the metrics server
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "715d8f3b.hades.tum.de",
	}

	if nsConfig.WatchNamespace != "" {
		mgrOpts.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				nsConfig.WatchNamespace: {},
			},
		}
	}

	// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
	// when the Manager ends. This requires the binary to immediately end when the
	// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
	// speeds up voluntary leader transitions as the new leader don't have to wait
	// LeaseDuration time first.
	//
	// In the default scaffold provided, the program ends immediately after
	// the manager stops, so would be fine to enable this option. However,
	// if you are doing or is intended to do any operation such as perform cleanups
	// after the manager stops then its usage might be unsafe.
	// LeaderElectionReleaseOnCancel: true,
	cfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(cfg, mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var natsConfig NCConfig
	utils.LoadConfig(&natsConfig)
	nc, err := utils.SetupNatsConnection(natsConfig.NatsConfig)
	if err != nil {
		setupLog.Error(err, "unable to setup NATS Connection")
		os.Exit(1)
	}

	publisher, err := log.NewNATSPublisher(nc)
	if err != nil {
		setupLog.Error(err, "unable to create NATS publisher")
		os.Exit(1)
	}

	kcs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "unable to init kubernetes clientset")
		os.Exit(1)
	}

	if err := (&controller.BuildJobReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		K8sClient:        kcs,
		NatsConnection:   nc,
		DeleteOnComplete: delOnComplete,
		Publisher:        *publisher,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BuildJob")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
