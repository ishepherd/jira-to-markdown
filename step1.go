package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"

	cli "github.com/jawher/mow.cli"
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
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	// TODO we might be good to do all operations on f?
	f2, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f2.Close()

	r := bufio.NewReader(f)
	d := xml.NewDecoder(r)
	d.Strict = false

enterRootElement:
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}
		// switch et := t.(type) {
		// case xml.StartElement:
		// 	if et.Name.Local != "entity-engine-xml" || et.Name.Space != "" || len(et.Attr) > 0 {
		// 		panic(fmt.Sprintf("wanted root element <entity-engine-xml>, got %v", et))
		// 	}
		// 	break enterRootElement
		// }
		et, ok := t.(xml.StartElement)
		if ok {
			if et.Name.Local != "entity-engine-xml" || et.Name.Space != "" {
				panic(fmt.Sprintf("wanted root element <entity-engine-xml>, got %v", et))
			}
			break enterRootElement
		}
	}

	i := 0
	// Now save each child
	for {
		var (
			startPos int64
			start    xml.StartElement
		)
	enterChildElement:
		for {
			startPos = d.InputOffset()
			t, err := d.Token()
			if err != nil {
				return err
			}
			var ok bool
			start, ok = t.(xml.StartElement)
			if ok {
				break enterChildElement
			}
		}
		err = d.Skip()
		if err != nil {
			return err
		}
		endPos := d.InputOffset()
		// fmt.Printf("Encountered %v from %v to %v\n", start, startPos, endPos)
		newOffset, err := f2.Seek(startPos, 0)
		if err != nil {
			return err
		}
		if newOffset != startPos {
			panic(fmt.Sprintf("wanted %v got %v", startPos, newOffset))
		}
		childContent := make([]byte, endPos-startPos)
		n, err := f2.Read(childContent)
		if err != nil {
			return err
		}
		if n != len(childContent) {
			panic(fmt.Sprintf("wanted %v got %v", len(childContent), n))
		}
		path := fmt.Sprintf("_tmp/%v", start.Name.Local)
		if _, err = os.Stat(path); os.IsNotExist(err) {
			err = os.MkdirAll(path, 0700) // Create your file
		}
		if err != nil {
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%v/%v.xml", path, i), childContent, 0644)
		if err != nil {
			return err
		}
		i++
	}
	// TODO after some time this fails with
	//   XML syntax error on line 4805102: illegal character code U+001D
}
