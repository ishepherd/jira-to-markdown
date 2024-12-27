package main

import (
	"flag"
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
	return nil
}
