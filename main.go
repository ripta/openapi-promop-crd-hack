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

var (
	// crdNames correspond to the internal OpenAPI definition names of the
	// CRDs we're interested in. Dependencies do not need to be listed, as
	// they will be calculated on build. The list are the keys of the map
	// returned from the function GetOpenAPIDefinitions at:
	//
	// https://github.com/coreos/prometheus-operator/blob/master/pkg/apis/monitoring/v1/openapi_generated.go
	crdNames = []string{
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.Alertmanager",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.PodMonitor",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.Prometheus",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.PrometheusRule",
		"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1.ServiceMonitor",
	}
)

func loadVanilla(path string) spec.Swagger {
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
	return vanilla
}

func main() {
	// load the swagger.json from kubernetes, which we're using here as a hack
	// to provide any references not already included by openapi-gen in the
	// monitoring.v1 API group.
	vanilla := loadVanilla("swagger.json")

	// use the same naming scheme that kubernetes uses, i.e., reverse domain
	// group, followed by version, then kind
	namer := openapi.NewDefinitionNamer(runtime.NewScheme())

	// construct a getter that uses monitoring.v1 as the primary definition,
	// falling back onto vanilla definitions
	definitionGetter := mergedDefinitions(monitoringv1.GetOpenAPIDefinitions, vanilla.Definitions, namer)

	cfg := &common.Config{
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title:   "Prometheus Operator CRD OpenAPI",
				Version: "v1",
			},
		},
		GetDefinitions:    definitionGetter,
		GetDefinitionName: namer.GetDefinitionName,
	}

	// generate swagger for our CRD names
	swag, err := builder.BuildOpenAPIDefinitionsForResources(cfg, crdNames...)
	if err != nil {
		log.Fatalf("cannot build openapi definitions: %+v", err)
	}

	// reëmit the new schema as json
	j, err := json.Marshal(swag)
	if err != nil {
		log.Fatalf("cannot marshal: %+v", err)
	}
	fmt.Println(string(j))
}

func mergedDefinitions(primary common.GetOpenAPIDefinitions, fallback spec.Definitions, namer *openapi.DefinitionNamer) common.GetOpenAPIDefinitions {
	all := map[string]common.OpenAPIDefinition{}
	return func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
		// This allows us to insert extra logic during callback processing,
		// by inserting the schema from the fallback into "all". Technically
		// not fully correct, because if the schema in question doesn't exist
		// in fallback, we continue anyway and hope that the primary definition
		// will have provided the schema (it would error out if not the case).
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
		// Copy whatever primary schemas as-is. References are resolved
		// during this primary(ref2) call.
		//
		// Doing the operations in this order does mean that only referenced
		// schemas from the fallback will be included in the final output.
		for k, v := range primary(ref2) {
			all[k] = v
		}
		return all
	}
}
