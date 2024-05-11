package notify

type Event string

const (
	EventTriggerSchedule Event = "trigger"
	EventSuccess         Event = "success"
	EventError           Event = "error"
)

type Notify struct {
	Events []Event `hcl:"events"`
}
