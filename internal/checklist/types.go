package checklist

// ChecklistType is the classification of a checklist.
type ChecklistType string

const (
	TypeNormal    ChecklistType = "normal"
	TypeEmergency ChecklistType = "emergency"
)

// ItemType describes how an item should be actioned.
type ItemType string

const (
	ItemDo    ItemType = "do"
	ItemCheck ItemType = "check"
)

// AutomationStep is one action that fires on item or checklist completion.
// Shell and Webhook may both be set; they run in that order.
type AutomationStep struct {
	Shell   string `yaml:"shell"`
	Webhook string `yaml:"webhook"`
}

// ConditionBranch defines what happens when a condition item is answered.
type ConditionBranch struct {
	Skip             bool   `yaml:"skip"`              // skip this item, continue checklist
	TriggerChecklist string `yaml:"trigger_checklist"` // start another checklist
	Abort            bool   `yaml:"abort"`             // abandon current checklist immediately
}

// Condition makes an item a yes/no decision point.
type Condition struct {
	IfYes *ConditionBranch `yaml:"if_yes"`
	IfNo  *ConditionBranch `yaml:"if_no"`
}

// Schedule defines when a checklist should be suggested on startup.
type Schedule struct {
	Frequency string `yaml:"frequency"` // daily | weekly | monthly
	On        string `yaml:"on"`        // weekday name for weekly (e.g. "monday")
	Period    string `yaml:"period"`    // morning | afternoon | evening (hint only)
	Cooldown  string `yaml:"cooldown"`  // minimum gap between runs, e.g. "7d"
}

// Item is a single checklist step.
type Item struct {
	ID         string           `yaml:"id"`
	Label      string           `yaml:"label"`
	Response   string           `yaml:"response"`
	Note       string           `yaml:"note"`
	Type       ItemType         `yaml:"type"`
	NAAllowed  bool             `yaml:"na_allowed"`
	OnComplete []AutomationStep `yaml:"on_complete"`
	Condition  *Condition       `yaml:"condition"`
}

// Phase groups items under a named heading.
type Phase struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

// Checklist is the top-level structure parsed from a YAML file.
type Checklist struct {
	Name             string           `yaml:"name"`
	Description      string           `yaml:"description"`
	Version          string           `yaml:"version"`
	Type             ChecklistType    `yaml:"type"`
	Phases           []Phase          `yaml:"phases"`
	Items            []Item           `yaml:"items"`
	OnComplete       []AutomationStep `yaml:"on_complete"`
	TriggerChecklist string           `yaml:"trigger_checklist"` // starts after this checklist completes
	Schedule         *Schedule        `yaml:"schedule"`
}

// FlatItem pairs an Item with its phase context and global position.
type FlatItem struct {
	Item      Item
	PhaseName string
	GlobalIdx int
}

// Flatten returns all items in order with phase information attached.
// If Phases is non-empty, items are drawn from each phase in order.
// Otherwise the top-level Items slice is used.
func (c *Checklist) Flatten() []FlatItem {
	var flat []FlatItem

	if len(c.Phases) > 0 {
		idx := 0
		for _, ph := range c.Phases {
			for _, item := range ph.Items {
				flat = append(flat, FlatItem{
					Item:      item,
					PhaseName: ph.Name,
					GlobalIdx: idx,
				})
				idx++
			}
		}
	} else {
		for i, item := range c.Items {
			flat = append(flat, FlatItem{
				Item:      item,
				PhaseName: "",
				GlobalIdx: i,
			})
		}
	}

	return flat
}

