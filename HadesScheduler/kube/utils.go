package kube

type K8sConfig struct {
	HadesCInamespace string `env:"HADES_CI_NAMESPACE" envDefault:"hades-ci"`
}
