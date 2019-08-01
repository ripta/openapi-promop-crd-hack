package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-openapi/jsonreference"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
)

const (
	definitionPrefix = "#/definitions/"
)

var buildDefinitions sync.Once
var definitions map[string]common.OpenAPIDefinition
var moswag *spec.Swagger
var namer *openapi.DefinitionNamer

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

	// obj := monitoringv1.DefaultCrdKinds.Prometheus
	namer := openapi.NewDefinitionNamer(runtime.NewScheme())
	cfg := &common.Config{
		ProtocolList: []string{"https"},
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title:   "Prometheus Operator CRD OpenAPI",
				Version: "v1",
			},
		},
		// CommonResponses: map[int]spec.Response{
		// 	401: {
		// 		ResponseProps: spec.ResponseProps{
		// 			Description: "Unauthorized",
		// 		},
		// 	},
		// },
		GetDefinitions: mergedDefinitions(monitoringv1.GetOpenAPIDefinitions, vanilla.Definitions, namer),
		// GetDefinitions:    mergedDefinitions(monitoringv1.GetOpenAPIDefinitions),
		GetDefinitionName: namer.GetDefinitionName,
		// GetDefinitionName: func(name string) (string, spec.Extensions) {
		// 	buildDefinitions.Do(buildDefinitionsFunc)
		// return namer.GetDefinitionName(name)
		// },
		// GetOperationIDAndTags: openapi.GetOperationIDAndTags,
	}

	// var def common.OpenAPIDefinition
	// def, _ := builder.BuildOpenAPIDefinitionsForResource(obj, cfg)
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
	// defs := monitoringv1.GetOpenAPIDefinitions(jsonRef)
	// j, err := json.Marshal(defs)
	if err != nil {
		log.Fatalf("cannot marshal: %+v", err)
	}
	fmt.Println(string(j))
}

func jsonRef(name string) spec.Ref {
	// return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(name))
	// return spec.MustCreateRef("#/definitions/" + name)
	return spec.Ref{
		Ref: jsonreference.MustCreateRef("#/definitions/" + name),
	}
}

// func buildDefinitionsFunc() {
// 	namer = openapi.NewDefinitionNamer(runtime.NewScheme())
// 	definitions = generatedopenapi.GetOpenAPIDefinitions(func(name string) spec.Ref {
// 		defName, _ := namer.GetDefinitionName(name)
// 		return spec.MustCreateRef(definitionPrefix + common.EscapeJsonPointer(defName))
// 	})
// }

func definitionGenerator(defs spec.Definitions) common.GetOpenAPIDefinitions {
	return func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
		oads := map[string]common.OpenAPIDefinition{}
		for k, v := range defs {
			oads[k] = common.OpenAPIDefinition{
				Schema: v,
			}
		}
		return oads
	}
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

		all["k8s.io/apimachinery/pkg/util/intstr.IntOrString"] = common.OpenAPIDefinition{}
		return all
	}
}
