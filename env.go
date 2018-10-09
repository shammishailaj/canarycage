package cage

import (
	"encoding/base64"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"math"
	"os"
	"path/filepath"
)

type Envars struct {
	_                           struct{} `type:"struct"`
	Region                      *string  `json:"region" type:"string"`
	Cluster                     *string  `json:"cluster" type:"string" required:"true"`
	NextServiceName             *string  `json:"nextServiceName" type:"string" required:"true"`
	CurrentServiceName          *string  `json:"currentServiceName" type:"string" required:"true"`
	NextServiceDefinitionBase64 *string  `json:"nextServiceDefinitionBase64" type:"string"`
	NextTaskDefinitionBase64    *string  `json:"nextTaskDefinitionBase64" type:"string"`
	NextTaskDefinitionArn       *string  `json:"nextTaskDefinitionArn" type:"string"`
	AvailabilityThreshold       *float64 `json:"availabilityThreshold" type:"double"`
	ResponseTimeThreshold       *float64 `json:"responseTimeThreshold" type:"double"`
	RollOutPeriod               *int64   `json:"rollOutPeriod" type:"integer"`
	SkipCanary                  *bool    `json:"skipCanary" type:"bool"`
}

// required
const ClusterKey = "CAGE_ECS_CLUSTER"
const NextServiceNameKey = "CAGE_NEXT_SERVICE_NAME"
const CurrentServiceNameKey = "CAGE_CURRENT_SERVICE_NAME"

// either required
const NextTaskDefinitionBase64Key = "CAGE_NEXT_TASK_DEFINITION_BASE64"
const NextTaskDefinitionArnKey = "CAGE_NEXT_TASK_DEFINITION_ARN"

// optional
const ConfigKey = "CAGE_CONFIG"
const NextServiceDefinitionBase64Key = "CAGE_NEXT_SERVICE_DEFINITION_BASE64"
const RegionKey = "CAGE_AWS_REGION"
const AvailabilityThresholdKey = "CAGE_AVAILABILITY_THRESHOLD"
const ResponseTimeThresholdKey = "CAGE_RESPONSE_TIME_THRESHOLD"
const RollOutPeriodKey = "CAGE_ROLL_OUT_PERIOD"
const SkipCanaryKey = "CAGE_SKIP_CANARY"

const kAvailabilityThresholdDefaultValue = 0.9970
const kResponseTimeThresholdDefaultValue = 1.0
const kRollOutPeriodDefaultValue = 300
const kDefaultRegion = "us-west-2"

func isEmpty(o *string) bool {
	return o == nil || *o == ""
}

func EnsureEnvars(
	dest *Envars,
) (error) {
	// required
	if isEmpty(dest.Cluster) {
		return NewErrorf("--cluster [%s] is required", ClusterKey)
	} else if isEmpty(dest.CurrentServiceName) {
		return NewErrorf("--currentServiceName [%s] is required", CurrentServiceNameKey)
	} else if isEmpty(dest.NextServiceName) {
		return NewErrorf("--nextServiceName [%s] is required", NextServiceNameKey)
	}
	if isEmpty(dest.NextTaskDefinitionArn) && isEmpty(dest.NextTaskDefinitionBase64) {
		return NewErrorf("--nextTaskDefinitionArn or --nextTaskDefinitionBase64 must be provided")
	}
	if isEmpty(dest.Region) {
		dest.Region = aws.String(kDefaultRegion)
	}
	if dest.AvailabilityThreshold == nil {
		dest.AvailabilityThreshold = aws.Float64(kAvailabilityThresholdDefaultValue)
	}
	if avl := *dest.AvailabilityThreshold; !(0.0 <= avl && avl <= 1.0) {
		return NewErrorf("--availabilityThreshold [%s] must be between 0 and 1, but got '%f'", AvailabilityThresholdKey, avl)
	}
	if dest.ResponseTimeThreshold == nil {
		dest.ResponseTimeThreshold = aws.Float64(kResponseTimeThresholdDefaultValue)
	}
	if rsp := *dest.ResponseTimeThreshold; !(0 < rsp && rsp <= 300) {
		return NewErrorf("--responseTimeThreshold [%s] must be greater than 0, but got '%f'", ResponseTimeThresholdKey, rsp)
	}
	// sec
	if dest.RollOutPeriod == nil {
		dest.RollOutPeriod = aws.Int64(kRollOutPeriodDefaultValue)
	}
	if period := *dest.RollOutPeriod; !(60 <= period && float64(period) != math.NaN() && float64(period) != math.Inf(0)) {
		return NewErrorf("--rollOutPeriod [%s] must be lesser than 60, but got '%d'", RollOutPeriodKey, period)
	}
	if dest.SkipCanary == nil {
		dest.SkipCanary = aws.Bool(false)
	}
	return nil
}

func (e *Envars) LoadFromFiles(dir string) error {
	svcPath := filepath.Join(dir, "service.json")
	tdPath := filepath.Join(dir, "task-definition.json")
	_, noSvc := os.Stat(svcPath)
	_, noTd := os.Stat(tdPath)
	if noSvc != nil || noTd != nil {
		return NewErrorf("roll out context specified at '%s' but no 'service.json' or 'task-definition.json'", dir)
	}
	var (
		svc = &ecs.CreateServiceInput{}
		td = &ecs.RegisterTaskDefinitionInput{}
		svcBase64 string
		tdBase64 string
	)
	if d ,err := ReadAndUnmarshalJson(svcPath, svc); err != nil {
		return NewErrorf("failed to read and unmarshal service.json: %s", err)
	} else {
		svcBase64 = base64.StdEncoding.EncodeToString(d)
	}
	if d, err := ReadAndUnmarshalJson(tdPath, td); err != nil {
		return NewErrorf("failed to read and unmarshal task-definition.json: %s", err)
	} else {
		tdBase64 = base64.StdEncoding.EncodeToString(d)
	}
	e.Cluster = svc.Cluster
	e.NextServiceName = svc.ServiceName
	e.NextServiceDefinitionBase64 = &svcBase64
	e.NextTaskDefinitionBase64 = &tdBase64
	return nil
}

func ReadAndUnmarshalJson(path string, dest interface{}) ([]byte, error) {
	if d, err := ReadFileAndApplyEnvars(path); err != nil {
		return d, err
	} else {
		if err := json.Unmarshal(d, dest); err != nil {
			return d, err
		}
		return d, nil
	}
}