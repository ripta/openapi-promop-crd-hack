package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
)

const (
	definitionPrefix = "#/definitions/"
)

func main() {
	r, err := os.Open("swagger.json")
	if err != nil {
		log.Fatalf("cannot open swagger.json: %+v", err)
	}
	p, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatalf("cannot read swagger.json: %+v", err)
	}

	var vanilla spec.Swagger
	if err := json.Unmarshal(p, &vanilla); err != nil {
		log.Fatalf("cannot unmarshal swagger.json: %+v", err)
	}

	namer := openapi.NewDefinitionNamer(runtime.NewScheme())
	cfg := &common.Config{
		ProtocolList: []string{"https"},
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title:   "Prometheus Operator CRD OpenAPI",
				Version: "v1",
			},
		},
		GetDefinitions:    mergedDefinitions(monitoringv1.GetOpenAPIDefinitions, vanilla.Definitions, namer),
		GetDefinitionName: namer.GetDefinitionName,
	}

	names := []string{
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.Alertmanager",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.PodMonitor",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.Prometheus",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.PrometheusRule",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.ServiceMonitor",
	}
	swag, err := builder.BuildOpenAPIDefinitionsForResources(cfg, names...)
	if err != nil {
		log.Fatalf("cannot build openapi definitions: %+v", err)
	}

	j, err := json.Marshal(swag)
	if err != nil {
		log.Fatalf("cannot marshal: %+v", err)
	}
	fmt.Println(string(j))
}

func mergedDefinitions(orig common.GetOpenAPIDefinitions, fallback spec.Definitions, namer *openapi.DefinitionNamer) common.GetOpenAPIDefinitions {
	all := map[string]common.OpenAPIDefinition{}
	return func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
		ref2 := func(name string) spec.Ref {
			if _, ok := all[name]; !ok {
				stdName, _ := namer.GetDefinitionName(name)
				if v, ok2 := fallback[stdName]; ok2 {
					all[name] = common.OpenAPIDefinition{
						Schema: v,
					}
				}
			}
			return ref(name)
		}
		for k, v := range orig(ref2) {
			all[k] = v
		}
		return all
	}
}
