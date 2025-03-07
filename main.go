package main

import (
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/printers"

	"github.com/go-kure/kure/pkg/deployment"
)

func main() {
	depl := deployment.Create()
	y := printers.YAMLPrinter{}
	err := y.PrintObj(depl, os.Stdout)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
