package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	app := cli.App(flag.CommandLine.Name(), "step 1 - Split entities.xml")
	app.Spec = "FILE"
	fileName := app.StringArg("FILE", "", "path to entities.xml file")
	app.Action = func() {
		if err := run(*fileName); err != nil {
			log.Println(err)
			cli.Exit(1)
		}
	}
	if err := app.Run(os.Args); err != nil {
		log.Println(err)
		cli.Exit(1)
	}
}

func run(fileName string) error {
	// remainderFile is where we will write all the portions of the file we're NOT using
	// so we can peek through later for anything interesting.
	remainderFile := "_tmp/remainder.xml"
	err := ensureDirExists(remainderFile)
	if err != nil {
		return err
	}
	rem, err := os.Create(remainderFile)
	if err != nil {
		return err
	}
	defer rem.Close()
	remainder := bufio.NewWriterSize(rem, 128*1024)

	err = turnRecordsIntoFiles(fileName, "/Volumes/ramdisk/_tmp", remainder)
	if err != nil {
		remainder.Flush()
		return err
	}

	remainder.Flush()
	return nil
}

func turnRecordsIntoFiles(fileName string, tmpDir string, remainder *bufio.Writer) error {
	fs, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	d := xml.NewDecoder(r)
	d.Strict = false

	// TODO f2 is so we have a separate seek position that doesn't affect the bufio.Reader(f). Maybe it doesn't affect it anyway?
	f2, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f2.Close()

enterRootElement:
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}
		et, ok := t.(xml.StartElement)
		if ok {
			if et.Name.Local != "entity-engine-xml" || et.Name.Space != "" {
				panic(fmt.Sprintf("wanted root element <entity-engine-xml>, got %v", et))
			}
			break enterRootElement
		}
	}

	// The prologue we skipped includes XML comments that might be interesting
	// put them in the remainder
	endPrologue := d.InputOffset()
	prologue, err := readByteRange(f2, 0, endPrologue)
	if err != nil {
		return err
	}
	_, err = remainder.Write(prologue)
	if err != nil {
		return err
	}

	// Now save each child
	for {
		var (
			startSkipping int64 = d.InputOffset()
			startPos      int64
			start         xml.StartElement
			eof           bool
		)
	enterNextElement:
		// Skip to the next child element
		for {
			startPos = d.InputOffset()
			t, err := d.Token()
			if err != nil {
				if err == io.EOF {
					// no more elements
					eof = true
					startPos = fs.Size()
					break enterNextElement
				}
				return err
			}
			var ok bool
			start, ok = t.(xml.StartElement)
			if ok {
				break enterNextElement
			}
		}

		// Write what we skipped to the remainder file.
		skipped, err := readByteRange(f2, startSkipping, startPos)
		if err != nil {
			return err
		}
		_, err = remainder.Write(skipped)
		if err != nil {
			return err
		}

		if eof {
			return nil
		}

		// Skip again, to the matching end element
		err = d.Skip()
		if err != nil {
			return err
		}

		// Get the entire text of the element we skipped
		endPos := d.InputOffset()
		content, err := readByteRange(f2, startPos, endPos)
		if err != nil {
			return err
		}
		// If it is boring append it to the remainder file
		// Otherwise create a new file with this element
		filenameForElement := makeFilename(start)
		if filenameForElement == "" {
			_, err = remainder.Write(content)
			if err != nil {
				return err
			}
		} else {
			// Copy the element into a new file
			err = writeFile(fmt.Sprintf("%v/%v", tmpDir, filenameForElement), content)
			if err != nil {
				return err
			}
		}
	}
}

var (
	i int
)

func makeFilename(el xml.StartElement) string {
	_, isBoring := boringElements[el.Name.Local]
	if isBoring {
		return ""
	}
	attrs := makeAttributes(el)

	makeId := func() string {
		if attrs.contains("id") {
			return attrs.get("id")
		}
		// fallback
		id := fmt.Sprintf("_%v", i)
		i++
		return id
	}

	if el.Name.Local == "ChangeGroup" {
		// if we let these enter the next if block, it would successfully put them under the Issue
		// however then the contained ChangeItems won't find them
		// TODO at end, move ChangeGroup directories under related Issues
		// TODO anything else that belongs in a 3 level hierarchy?
		return fmt.Sprintf("ChangeGroup/%v/issue-%v.xml",
			attrs.get("id"), attrs.get("issue"))
	} else if el.Name.Local == "ChangeItem" {
		return fmt.Sprintf("ChangeGroup/%v/%v/%v.xml",
			attrs.get("group"), el.Name.Local, makeId())
	}

	if el.Name.Local == "Issue" {
		// the actual tickets
		return fmt.Sprintf("Issue/%v/%v-%v.xml",
			attrs.get("id"), attrs.get("projectKey"), attrs.get("number"))
		// below cases: all (that we want) that joins directly to an Issue
		// Put it under the Issue's directory
		// Action (comment), FileAttachment (attachment metadata), etc
	} else if attrs.contains("issue") {
		return fmt.Sprintf("Issue/%v/%v/%v.xml",
			attrs.get("issue"), el.Name.Local, makeId())
	} else if attrs.contains("issue_id") {
		return fmt.Sprintf("Issue/%v/%v/%v.xml",
			attrs.get("issue_id"), el.Name.Local, makeId())
	} else if el.Name.Local == "IssueView" {
		return fmt.Sprintf("Issue/%v/%v/%v.xml",
			attrs.get("id"), el.Name.Local, makeId())
	} else if attrs.contains("issueId") {
		return fmt.Sprintf("Issue/%v/%v/%v.xml",
			attrs.get("issueId"), el.Name.Local, makeId())
	} else if attrs.contains("issueid") {
		return fmt.Sprintf("Issue/%v/%v/%v.xml",
			attrs.get("issueid"), el.Name.Local, makeId())
	}

	// User - ApplicationUser
	// for some users the ids match eg 14143, 14149
	// for latest user the ApplicationUser just has a guid - same in both userKey and lowerUserName

	if el.Name.Local == "Project" {
		return fmt.Sprintf("Project/%v/%v.xml",
			attrs.get("id"), attrs.get("key"))
	} else if attrs.contains("projectId") {
		return fmt.Sprintf("Project/%v/%v/%v.xml",
			attrs.get("projectId"), el.Name.Local, makeId())
	}

	if el.Name.Local == "IssueLinkType" {
		return fmt.Sprintf("IssueLinkType/%v/%v.xml",
			attrs.get("id"), attrs.get("linkname"))
	} else if attrs.contains("linktype") {
		return fmt.Sprintf("IssueLinkType/%v/%v/%v.xml",
			attrs.get("linktype"), el.Name.Local, makeId())
	}

	if el.Name.Local == "IssueType" {
		return fmt.Sprintf("IssueType/%v/%v.xml",
			attrs.get("id"), attrs.get("name"))
	} else if attrs.contains("issueTypeId") {
		return fmt.Sprintf("IssueType/%v/%v/%v.xml",
			attrs.get("issueTypeId"), el.Name.Local, makeId())
	}

	if el.Name.Local == "AuditLog" {
		return fmt.Sprintf("AuditLog/%v/%v.xml",
			attrs.get("id"), attrs.get("id"))
	} else if attrs.contains("logId") {
		return fmt.Sprintf("AuditLog/%v/%v/%v.xml",
			attrs.get("logId"), el.Name.Local, makeId())
	}

	if el.Name.Local == "CustomField" {
		return fmt.Sprintf("CustomField/%v/%v.xml",
			attrs.get("id"), attrs.get("name"))
	} else if attrs.contains("customfield") {
		return fmt.Sprintf("CustomField/%v/%v/%v.xml",
			attrs.get("customfield"), el.Name.Local, makeId())
	} else if attrs.contains("customField") {
		return fmt.Sprintf("CustomField/%v/%v/%v.xml",
			attrs.get("customField"), el.Name.Local, makeId())
	} else if attrs.contains("customfieldId") {
		return fmt.Sprintf("CustomField/%v/%v/%v.xml",
			attrs.get("customfieldId"), el.Name.Local, makeId())
	}

	return fmt.Sprintf("%v/%v.xml", el.Name.Local, makeId())
}

type attributes struct {
	element    xml.StartElement
	theAttribs map[string]xml.Attr
}

func makeAttributes(el xml.StartElement) attributes {
	return attributes{
		element:    el,
		theAttribs: makeMap(el.Attr, func(a xml.Attr) string { return a.Name.Local }),
	}
}

var (
	// fileSafeRegexp matches values we accept as paths/names in the filesystem.
	fileSafeRegexp *regexp.Regexp = regexp.MustCompile(`^[\w \[\]()\-&!]+$`)
)

func (a attributes) get(key string) string {
	result, ok := a.theAttribs[key]
	if !ok {
		panic(fmt.Sprintf("oops I thought the %v had a %v attribute. %v", a.element.Name, key, a))
	}
	if !fileSafeRegexp.MatchString(result.Value) {
		panic(fmt.Sprintf("oops I thought every %v would be safe for the filesystem. %s", result.Value, a))
	}
	return result.Value
}

func (a attributes) contains(key string) bool {
	_, ok := a.theAttribs[key]
	return ok
}

func makeMap[K comparable, V any](src []V, key func(V) K) map[K]V {
	var result = make(map[K]V)
	for _, v := range src {
		result[key(v)] = v
	}
	return result
}

func writeFile(name string, content []byte) error {
	err := ensureDirExists(name)
	if err != nil {
		return err
	}
	err = os.WriteFile(name, content, 0644)
	if err != nil {
		return err
	}
	return nil
}

func ensureDirExists(name string) error {
	dir := filepath.Dir(name)
	_, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
	}
	if err != nil {
		return err
	}
	return nil
}

var (
	boringElements map[string]struct{} = map[string]struct{}{
		"AdHocUpgradeTaskHistory":        {},
		"AffectsVersion":                 {},
		"AttachmentPanel":                {},
		"BaseHierarchyLevel":             {},
		"Board":                          {},
		"BoardProject":                   {},
		"ClusteredJob":                   {},
		"ColumnLayout":                   {},
		"ColumnLayoutItem":               {},
		"ConfigurationContext":           {},
		"Directory":                      {},
		"DirectoryAttribute":             {},
		"DirectoryOperation":             {},
		"DraftWorkflow":                  {},
		"DraftWorkflowScheme":            {},
		"DraftWorkflowSchemeEntity":      {},
		"EntityProperty":                 {}, // build status
		"EntityTranslation":              {},
		"FavouriteAssociations":          {},
		"Feature":                        {},
		"FieldConfigScheme":              {},
		"FieldConfigSchemeIssueType":     {},
		"FieldConfiguration":             {},
		"FieldLayout":                    {},
		"FieldLayoutItem":                {},
		"FieldLayoutScheme":              {},
		"FieldLayoutSchemeEntity":        {},
		"FieldScreen":                    {},
		"FieldScreenLayoutItem":          {},
		"FieldScreenScheme":              {},
		"FieldScreenSchemeItem":          {},
		"FieldScreenTab":                 {},
		"FieldScreenWorkflowTransitions": {},
		"GadgetUserPreference":           {},
		"GenericConfiguration":           {},
		"GlobalPermissionEntry":          {},
		"Group":                          {},
		"GroupAttribute":                 {},
		"HierarchyLevel":                 {},
		"IssueFieldOption":               {},
		"IssueFieldOptionScope":          {},
		"IssueLayoutAssociation":         {},
		"IssueLayoutFieldOperations":     {},
		"IssueLayoutItemPosition":        {},
		"IssueSecurityScheme":            {},
		"IssueTypeHierarchy":             {},
		"IssueTypeHierarchyAssociation":  {},
		"IssueTypeScreenScheme":          {},
		"IssueTypeScreenSchemeEntity":    {},
		"LicenseRoleDefault":             {},
		"LicenseRoleGroup":               {},
		"ListenerConfig":                 {},
		"MailServer":                     {},
		"ModuleStatus":                   {},
		"Notification":                   {},
		"NotificationInstance":           {},
		"NotificationScheme":             {},
		"OAuthConsumerToken":             {},
		"OAuthServiceProviderToken":      {},
		"OptionConfiguration":            {},
		"PermissionScheme":               {},
		"PluginVersion":                  {},
		"PortalPage":                     {}, // dashboards
		"PortletConfiguration":           {},
		"ProjectRole":                    {},
		"ProjectRoleActor":               {},
		"SchemeIssueSecurities":          {},
		"SchemeIssueSecurityLevels":      {},
		"SchemePermissions":              {},
		"SearchRequest":                  {}, // filters
		"SequenceValueItem":              {},
		"ServiceConfig":                  {},
		"SharePermissions":               {},
		"StatusCategoryChange":           {},
		"UpgradeHistory":                 {},
		"UpgradeTaskHistory":             {},
		"UpgradeTaskHistoryAuditLog":     {},
		"UpgradeVersionHistory":          {},
		"Workflow":                       {},
		"WorkflowScheme":                 {},
		"WorkflowSchemeEntity":           {},
		"WorkflowStatuses":               {},
		// 'OS' stuff seems to be a generic extension mechanism
		// content seems to refer to workflows, filters, and plugins. See 'OSPropertyText'
		"OSCurrentStep":     {},
		"OSCurrentStepPrev": {},
		"OSHistoryStep":     {},
		"OSHistoryStepPrev": {},
		"OSPropertyDate":    {},
		"OSPropertyEntry":   {},
		"OSPropertyNumber":  {},
		"OSPropertyString":  {},
		"OSPropertyText":    {},
		"OSWorkflowEntry":   {},
	}
)

func readByteRange(f *os.File, startPos int64, endPos int64) ([]byte, error) {
	newOffset, err := f.Seek(startPos, 0)
	if err != nil {
		return nil, err
	}
	if newOffset != startPos {
		panic(fmt.Sprintf("seek: wanted %v got %v", startPos, newOffset))
	}
	childContent := make([]byte, endPos-startPos)
	n, err := f.Read(childContent)
	if err != nil {
		return nil, err
	}
	if n != len(childContent) {
		panic(fmt.Sprintf("read: wanted %v got %v", len(childContent), n))
	}
	return childContent, nil
}
