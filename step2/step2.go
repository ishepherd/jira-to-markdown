package main

import (
	"encoding/xml"
	"flag"
	"fmt"
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
		return nil, err
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
	normalizeIntoElements(&issue.SummaryAttr, &issue.Summary)
	normalizeIntoElements(&issue.DescriptionAttr, &issue.Description)
	normalizeIntoElements(&issue.EnvironmentAttr, &issue.Environment)
	// if issue.Summary != "" {
	// 	issue.Summary = issue.SummaryAttr
	// 	issue.SummaryAttr = ""
	// }

	return err
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
		return err
	}

	// Check if there are any [UnknownNodes] in [data].
	t := reflect.TypeOf(target).Elem() // Elem() derefs pointer
	v := reflect.ValueOf(target).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Name == "UnknownNodes" {
			fv := v.Field(i)
			unk := fv.Interface().(UnknownNodes)
			mustBeNoUnknownNodes(unk, filename)
			return nil
		}
	}

	panic(fmt.Sprintf("no UnknownNodes support on %s", t))
}

func mustBeNoUnknownNodes(u UnknownNodes, filename string) {
	// Atlassian's JIRA backup process appears to only serialize fields if they contain something.
	// (As child elements if they contain special chars eg newlines;
	// as attributes otherwise).
	//
	// So any unmarshal may encounter an attribute/element we haven't seen before.
	//
	// So detect if any of this has happened and panic to make me review and add support.

	if len(u.Unknown) != 0 {
		panic(fmt.Sprintf("child elements would have been discarded, check this is desired. got %v unknown child elements in %s", len(u.Unknown), filename))
	}
	if len(u.UnknownAttrs) != 0 {
		panic(fmt.Sprintf("attributes would have been discarded, check this is desired. got unknown attribs %v in %s", u.UnknownAttrs, filename))
	}
	if strings.TrimSpace(u.CharData) != "" {
		panic(fmt.Sprintf("chardata would have been discarded, check this is desired. got %s in %s", u.UnknownAttrs, filename))
	}
	if strings.TrimSpace(u.Comment) != "" {
		panic(fmt.Sprintf("comment would have been discarded, check this is desired. got %s in %s", u.Comment, filename))
	}
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
				panic(fmt.Sprintf("Expected there to be one file ending %s at %s, but there are %s and %s",
					suffix, issueDir, found, file.Name()))
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
