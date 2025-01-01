package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"strings"

	cli "github.com/jawher/mow.cli"
)

func main() {
	app := cli.App(flag.CommandLine.Name(), `step 2 - Condense each JIRA issue's dir into a single XML file`)
	app.Spec = "[-o]"
	var (
		outputDir = app.StringOpt("o outputDir", "/Volumes/ramdisk/_tmp", "the output files location from step 1")
	)
	app.Action = func() {
		if err := run(*outputDir); err != nil {
			log.Println(err)
			cli.Exit(1)
		}
	}
	if err := app.Run(os.Args); err != nil {
		// bad args
		log.Println(err)
		cli.Exit(1)
	}
}

func run(outputDir string) error {
	issueDirs, err := os.ReadDir(fmt.Sprintf("%v/Issue", outputDir))
	if err != nil {
		return err
	}
	for _, issueDir := range issueDirs {
		if !issueDir.IsDir() {
			continue
		}
		task, err := createTaskData(fmt.Sprintf("%v/Issue/%v", outputDir, issueDir.Name()))
		if err != nil {
			return err
		}
		if task != nil {
			err = task.run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type taskData struct {
	issueDir     string
	issueXmlFile string
}

func createTaskData(issueDir string) (*taskData, error) {
	issueXmlFile, err := findOneFile(issueDir, ".xml")
	if err != nil {
		return nil, fmt.Errorf("%v: %w", issueDir, err)
	}
	if issueXmlFile == "" {
		return nil, nil
	}
	return &taskData{
		issueDir:     issueDir,
		issueXmlFile: issueXmlFile,
	}, nil
}

func (t taskData) run() error {
	// Unmarshal the issue XML
	b, err := os.ReadFile(t.issueXmlFile)
	if err != nil {
		return err
	}
	var issue Issue
	err = unmarshal(b, &issue, t.issueXmlFile)
	if err != nil {
		return err
	}
	normalizeIntoElements(&issue.SummaryAttr, &issue.Summary)
	normalizeIntoElements(&issue.DescriptionAttr, &issue.Description)
	normalizeIntoElements(&issue.EnvironmentAttr, &issue.Environment)
	output := OutputIssue{
		Issue: issue,
	}

	// Visit the child directories
	children, err := os.ReadDir(t.issueDir)
	if err != nil {
		return err
	}
	for _, child := range children {
		if !child.IsDir() {
			continue
		}
		switch child.Name() {
		case "Action":
			actions, err := t.readActions(child)
			if err != nil {
				return err
			}
			output.Actions = actions
		// case "ChangeGroup":
		// 	cg, err := t.readChangeGroup(child)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	output.ChangeGroup = cg
		default:
			// return fmt.Errorf("Unexpected child dir: %v/%v", t.issueDir, child.Name())
		}
	}
	return nil
}

func (t taskData) readActions(child fs.DirEntry) ([]OutputAction, error) {
	children, err := os.ReadDir(fmt.Sprintf("%v/%v", t.issueDir, child.Name()))
	if err != nil {
		return nil, err
	}
	actions := make([]OutputAction, len(children))
	for i, actionFile := range children {
		af := fmt.Sprintf("%v/%v/%v", t.issueDir, child.Name(), actionFile.Name())
		b, err := os.ReadFile(af)
		if err != nil {
			return actions, err
		}
		var action Action
		err = unmarshal(b, &action, af)
		if err != nil {
			return actions, err
		}
		normalizeIntoElements(&action.BodyAttr, &action.Body)
		actions[i] = OutputAction{Action: action}
	}
	// TODO sort the slice by date
	return actions, nil
}

func (t taskData) readChangeGroup(child fs.DirEntry) ([]OutputChangeGroup, error) {
	return []OutputChangeGroup{}, nil // TODO
}

func normalizeIntoElements(attr *string, elem *string) {
	// JIRA writes out attributes as elements instead if they contain newlines.
	// Where this has been seen to happen, move the attribute into the element
	if *elem == "" {
		*elem = *attr
		*attr = ""
	}
}

func unmarshal(data []byte, target any, filename string) error {
	err := xml.Unmarshal(data, &target)
	if err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}

	// Check if there are any [UnknownNodes] in [data].
	t := reflect.TypeOf(target).Elem() // Elem() derefs pointer
	v := reflect.ValueOf(target).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Name == "UnknownNodes" {
			fv := v.Field(i)
			unk := fv.Interface().(UnknownNodes)

			if err = checkNoUnknownNodes(unk); err != nil {
				return fmt.Errorf("%s: %w", filename, err)
			}
			return nil
		}
	}

	panic(fmt.Sprintf("no UnknownNodes support on %s", t))
}

func checkNoUnknownNodes(u UnknownNodes) error {
	// Atlassian's JIRA backup process appears to only serialize fields if they contain something.
	// (As child elements if they contain special chars eg newlines;
	// as attributes otherwise).
	//
	// So any unmarshal may encounter an attribute/element we haven't seen before.
	// So detect if any of this has happened.

	if len(u.Unknown) != 0 {
		return fmt.Errorf("%v child elements would have been discarded, check this is desired", len(u.Unknown))
	}
	if len(u.UnknownAttrs) != 0 {
		return fmt.Errorf("attributes would have been discarded, check this is desired: %v", u.UnknownAttrs)
	}
	if strings.TrimSpace(u.CharData) != "" {
		return fmt.Errorf("chardata would have been discarded, check this is desired: %s", u.CharData)
	}
	if strings.TrimSpace(u.Comment) != "" {
		return fmt.Errorf("comment would have been discarded, check this is desired: %s", u.Comment)
	}
	return nil
}

func findOneFile(issueDir string, suffix string) (string, error) {
	// os.Readdirfiles is said to be much faster, but needs opening and closing the dir as an os.File.
	files, err := os.ReadDir(issueDir)
	if err != nil {
		return "", err
	}
	var found string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), suffix) {
			if found != "" {
				return "", fmt.Errorf("Expected there to be one file ending %s at %s, but there are %s and %s",
					suffix, issueDir, found, file.Name())
			}
			found = file.Name()
		}
	}
	if found == "" {
		// TODO seen one Issue num for which there was nothing but Labels
		// test that we have some dangling Issue dirs at the edn which weren't converted and review then
		// 	panic(fmt.Sprintf("Expected there to be one file ending %s at %s, but there are none", suffix, issueDir))
		return "", nil
	}
	return fmt.Sprintf("%v/%v", issueDir, found), nil
}
