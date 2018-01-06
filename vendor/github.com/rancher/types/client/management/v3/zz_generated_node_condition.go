package client

const (
	NodeConditionType                    = "nodeCondition"
	NodeConditionFieldLastHeartbeatTime  = "lastHeartbeatTime"
	NodeConditionFieldLastTransitionTime = "lastTransitionTime"
	NodeConditionFieldMessage            = "message"
	NodeConditionFieldReason             = "reason"
	NodeConditionFieldStatus             = "status"
	NodeConditionFieldType               = "type"
)

type NodeCondition struct {
	LastHeartbeatTime  string `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
