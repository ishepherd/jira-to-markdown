package main

import "encoding/xml"

type Issue struct {
	UnknownNodes
	ProjectKey string `xml:"projectKey,attr"`
	Number     int    `xml:"number,attr"`
	Project    int    `xml:"project,attr"`
	Id         int    `xml:"id,attr"`

	Summary     string `xml:"summary"`
	SummaryAttr string `xml:"summary,attr"`

	Description     string `xml:"description"`
	DescriptionAttr string `xml:"description,attr"`

	Environment     string `xml:"environment"`
	EnvironmentAttr string `xml:"environment,attr"`

	Reporter                  string `xml:"reporter,attr"`
	Assignee                  string `xml:"assignee,attr"`
	Creator                   string `xml:"creator,attr"`
	Type                      int    `xml:"type,attr"`
	Priority                  int    `xml:"priority,attr"`
	Resolution                int    `xml:"resolution,attr"`
	Status                    int    `xml:"status,attr"`
	Created                   string `xml:"created,attr"`
	Updated                   string `xml:"updated,attr"`
	ResolutionDate            string `xml:"resolutiondate,attr"`
	DueDate                   string `xml:"duedate,attr"`
	Votes                     int    `xml:"votes,attr"`
	Watches                   int    `xml:"watches,attr"`
	WorkflowId                int    `xml:"workflowId,attr"`
	EffectiveSubtaskParentId  int    `xml:"effectiveSubtaskParentId,attr"`
	LifecycleState            string `xml:"lifecycleState,attr"`
	TimeOriginalEstimate      int    `xml:"timeoriginalestimate,attr"`
	TimeEstimate              int    `xml:"timeestimate,attr"`
	TimeSpent                 int    `xml:"timespent,attr"`
	DenormalisedSubtaskParent int    `xml:"denormalisedSubtaskParent,attr"`
	SubtaskParentId           int    `xml:"subtaskParentId,attr"`
	ReadExternal              bool   `xml:"read_external,attr"`
	SoftArchived              bool   `xml:"softArchived,attr"`
}

type Action struct {
	UnknownNodes
	Id     int    `xml:"id,attr"`
	Issue  int    `xml:"issue,attr"`
	Author string `xml:"author,attr"`
	Type   string `xml:"type,attr"`

	Body     string `xml:"body"`
	BodyAttr string `xml:"body,attr"`

	Created      string `xml:"created,attr"`
	UpdateAuthor string `xml:"updateauthor,attr"`
	Updated      string `xml:"updated,attr"`
}

type ChangeGroup struct {
	UnknownNodes
}

type UnknownNodes struct {
	Unknown      []any      `xml:",any"`
	UnknownAttrs []xml.Attr `xml:",any,attr"`
	CharData     string     `xml:",chardata"`
	Comment      string     `xml:",comment"`
}
