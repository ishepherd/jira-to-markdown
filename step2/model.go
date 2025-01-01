package main

type OutputIssue struct {
	Issue
	Actions      []OutputAction
	ChangeGroups []OutputChangeGroup
}

type OutputAction struct {
	Action
}

type OutputChangeGroup struct {
	ChangeGroup
}
