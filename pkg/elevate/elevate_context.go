package elevate

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/openshift/backplane-cli/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	elevateExtensionName             = "ElevateContext"
	elevateExtensionRetentionMinutes = 20
)

var (
	ModifyConfig          = clientcmd.ModifyConfig
	AskQuestionFromPrompt = utils.AskQuestionFromPrompt
)

type ElevateContext struct {
	Reasons  []string  `json:"reasons"`
	LastUsed time.Time `json:"lastUsed"`
}

///////////////////////////////////////////////////////////////////////
// runtime.Object interface func implementation for ElevateContext type

// DeepCopyObject creates a deep copy of the ElevateContext.
func (r *ElevateContext) DeepCopyObject() runtime.Object {
	return &ElevateContext{
		Reasons:  append([]string(nil), r.Reasons...),
		LastUsed: r.LastUsed,
	}
}

// GetObjectKind returns the schema.GroupVersionKind of the object.
func (r *ElevateContext) GetObjectKind() schema.ObjectKind {
	// return schema.EmptyObjectKind
	return &runtime.TypeMeta{
		Kind:       "ElevateContext",
		APIVersion: "v1",
	}
}

///////////////////////////////////////////////////////////////////////

// in some cases (mainly when config is created from json) the "ElevateContext Extension" is created as runtime.Unknow object
// instead of the desired ElevateContext, so we need to Unmarshal the raw definition in that case
func GetElevateContextReasons(config api.Config) []string {
	if currentContext := config.Contexts[config.CurrentContext]; currentContext != nil {
		var elevateContext *ElevateContext
		var ok bool
		if object := currentContext.Extensions[elevateExtensionName]; object != nil {
			//Let's first try to cast the extension object in ElevateContext
			if elevateContext, ok = object.(*ElevateContext); !ok {
				// and if it does not work, let's try cast the extension object in Unknown
				if unknownObject, ok := object.(*runtime.Unknown); ok {
					// and unmarshal the unknown raw JSON string into the ElevateContext
					_ = json.Unmarshal([]byte(unknownObject.Raw), &elevateContext)
				}
			}
			// We should keep the stored ElevateContext only if it is still valid
			if elevateContext != nil && time.Since(elevateContext.LastUsed) <= elevateExtensionRetentionMinutes*time.Minute {
				return elevateContext.Reasons
			}
		}
	}
	return []string{}
}

func ComputeElevateContextAndStoreToKubeConfigFileAndGetReasons(config api.Config, elevationReason string) ([]string, error) {
	currentCtx := config.Contexts[config.CurrentContext]
	if currentCtx == nil {
		return nil, errors.New("no current kubeconfig context")
	}

	// let's first retrieve previous elevateContext if any, and add any provided reason.
	elevationReasons := utils.AppendUniqNoneEmptyString(
		GetElevateContextReasons(config),
		elevationReason,
	)
	// if we still do not have reason, then let's try to have the reason from prompt
	if len(elevationReasons) == 0 {
		elevationReasons = utils.AppendUniqNoneEmptyString(
			elevationReasons,
			AskQuestionFromPrompt(fmt.Sprintf("Please enter a reason for elevation, it will be stored in current context for %d minutes : ", elevateExtensionRetentionMinutes)),
		)
	}
	// and raise an error if not possible
	if len(elevationReasons) == 0 {
		return nil, errors.New("please enter a reason for elevation")
	}

	// Store the ElevateContext in config current context Extensions map
	if currentCtx.Extensions == nil {
		currentCtx.Extensions = map[string]runtime.Object{}
	}
	currentCtx.Extensions[elevateExtensionName] = &ElevateContext{
		Reasons:  elevationReasons,
		LastUsed: time.Now(),
	}

	// Save the config to default path.
	configAccess := clientcmd.NewDefaultPathOptions()
	err := ModifyConfig(configAccess, config, true)

	return elevationReasons, err
}
